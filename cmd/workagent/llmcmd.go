package main

// LLM 步骤：把模型调用纳入证据链。
//
// 核心矛盾：大模型输出天然不可复现，而本项目的承诺是"结论可复现"。
// 解法是把一次调用拆成两半：
//
//   1) LLM 只做「抽取/翻译」——把非结构化输入变成结构化中间表示；
//   2) 判断与计算全部交给确定性代码。
//
// 并且把模型的原始响应**录下来**，verify 重放记录而非重放模型，
// 于是裁决过程仍然可复现。复现等级如实标注，绝不冒充。

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/WmjXiaoJun/workagent/internal/i18n"
	"github.com/WmjXiaoJun/workagent/internal/llm"
)

const (
	llmDir = "provenance/llm"
	runDir = "out/run"
)

// ---------------------------------------------------------------- models

// cmdModels 只做一件事：把内置服务商打成一张表。
// 默认**不发起任何网络请求** —— 打印表格的命令不该因为网络行为而失败。
// 需要探测本地端点时显式加 --probe。
func cmdModels(probe bool) int {
	fmt.Println(i18n.T("WorkAgent models —— 内置模型服务商（零第三方依赖）", "WorkAgent models — built-in providers (zero third-party dependencies)"))
	fmt.Println("")
	fmt.Printf("%-12s %-10s %-34s %-24s %s\n", "PROVIDER", "PROTOCOL", i18n.T("端点", "ENDPOINT"), i18n.T("密钥环境变量", "API KEY ENV"), i18n.T("状态", "STATUS"))
	fmt.Println(strings.Repeat("-", 118))
	for _, p := range llm.Profiles() {
		var status string
		switch {
		case p.KeyEnv == "":
			status = i18n.T("本地（无需密钥）", "local (no key needed)")
			if probe {
				if reachable(p.Endpoint()) {
					status = i18n.T("本地可用", "local, reachable")
				} else {
					status = i18n.T("未探测到", "not detected")
				}
			}
		case p.KeyPresent():
			status = i18n.T("密钥已配置", "key configured")
		default:
			status = i18n.T("缺少密钥", "key missing")
		}
		keyEnv := p.KeyEnv
		if keyEnv == "" {
			keyEnv = i18n.T("(无需)", "(none)")
		}
		fmt.Printf("%-12s %-10s %-34s %-24s %s\n", p.Name, p.Kind, truncate(p.Endpoint(), 33), keyEnv, status)
	}
	fmt.Println("")
	if !probe {
		fmt.Println(i18n.T("提示：加 --probe 可探测本地端点是否在跑（默认不发起任何网络请求）", "Tip: add --probe to check whether local endpoints are running (by default no network request is made)"))
		fmt.Println("")
	}
	fmt.Println(i18n.T("用法:", "Usage:"))
	fmt.Println("  workagent run --input <file> [--provider NAME] [--model NAME] [--replay]")
	fmt.Println("  workagent run --input <file> --provider deepseek --model deepseek-chat")
	fmt.Println(i18n.T("  workagent run --input <file> --replay        # 用已录制响应，离线可复现", "  workagent run --input <file> --replay        # replay a recorded response, offline and reproducible"))
	fmt.Println("")
	fmt.Println(i18n.T("任何 OpenAI 兼容端点都可用 custom profile：", "Any OpenAI-compatible endpoint works via the custom profile:"))
	fmt.Println("  set CUSTOM_BASE_URL=https://your-endpoint/v1  ;  set CUSTOM_API_KEY=xxx")
	fmt.Println("  workagent run --input <file> --provider custom --model your-model")
	return 0
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func reachable(endpoint string) bool {
	if endpoint == "" {
		return false
	}
	client := &http.Client{Timeout: 1500 * time.Millisecond}
	resp, err := client.Get(endpoint + "/models")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode < 500
}

// ---------------------------------------------------------------- run

// runLLMSteps 执行全部 LLM 步骤，返回富化后的输入文件路径与调用描述。
func runLLMSteps(root, caseID string, steps []any, inputObj any, replay bool, providerOverride, modelOverride string, record bool) (string, []map[string]any, error) {
	if caseID == "" {
		caseID = "default"
	}
	if len(steps) == 0 {
		return "", nil, nil
	}
	if err := os.MkdirAll(filepath.Join(root, llmDir), 0o755); err != nil {
		return "", nil, err
	}
	if err := os.MkdirAll(filepath.Join(root, runDir, "llm"), 0o755); err != nil {
		return "", nil, err
	}
	inputCanon, _ := canonicalJSON(inputObj)
	enriched := map[string]any{"input": inputObj, "llm": map[string]any{}}
	var descriptors []map[string]any

	for _, raw := range steps {
		st := asMap(raw)
		stepID := asString(st["id"])
		if stepID == "" {
			return "", nil, fmt.Errorf(i18n.T("llm_steps 缺少 id", "llm_steps is missing an id"))
		}
		// 条件执行：输入里没有指定字段就跳过该步骤。
		// 于是「结构化输入」可以零联网跑通，「自然语言输入」才需要模型。
		if cond := asString(st["when_input_has"]); cond != "" {
			v, ok := asMap(inputObj)[cond]
			if !ok || v == nil || asString(v) == "" {
				fmt.Printf(i18n.T("  [--]   %s: 跳过（输入中无 %s）\n", "  [--]   %s: skipped (input has no %s)\n"), stepID, cond)
				continue
			}
		}
		provider := asString(st["provider"])
		model := asString(st["model"])
		if providerOverride != "" {
			provider = providerOverride
		}
		if modelOverride != "" {
			model = modelOverride
		}
		promptFile := asString(st["prompt"])
		tplBytes, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(promptFile)))
		if err != nil {
			return "", nil, fmt.Errorf(i18n.T("步骤 %s 读不到 prompt 文件 %s: %v", "step %s cannot read prompt file %s: %v"), stepID, promptFile, err)
		}
		prompt := strings.ReplaceAll(string(tplBytes), "{{INPUT_JSON}}", string(inputCanon))
		promptDigest := sha256Bytes([]byte(prompt))

		// 录制必须按「场景」分目录：不同场景的步骤常常同名（都叫 extract），
		// 共用路径会互相覆盖，导致重放拿到别人的响应。
		recordPath := filepath.Join(root, llmDir, caseID, stepID+".json")
		var text, rawResp string
		var desc map[string]any
		var level string
		replayed := false

		if replay {
			rec, rerr := readJSON(recordPath)
			if rerr != nil {
				return "", nil, fmt.Errorf(i18n.T("步骤 %s 处于 replay 模式，但找不到录制文件 %s —— 请先在线运行一次", "step %s is in replay mode but no recording was found at %s — run it online once first"), stepID, recordPath)
			}
			rm := asMap(rec)
			if asString(rm["prompt_digest"]) != promptDigest {
				return "", nil, fmt.Errorf(i18n.T("步骤 %s 的 prompt 已变化，录制失效（记录 %s ≠ 当前 %s）—— 请重新录制",
					"the prompt for step %s has changed; the recording is stale (recorded %s != current %s) — record it again"),
					stepID, short(asString(rm["prompt_digest"])), short(promptDigest))
			}
			text = asString(rm["text"])
			rawResp = asString(rm["raw"])
			desc = asMap(rm["descriptor"])
			// 等级描述的是「这条证据是怎么产生的」，重放只是读取它 —— 不得降级。
			level = asString(rm["reproducibility"])
			if level == "" {
				level = "L1-record-replay"
			}
			replayed = true
			fmt.Printf(i18n.T("  [--]   %s: replay（使用录制响应，未联网）\n", "  [--]   %s: replay (using the recorded response, offline)\n"), stepID)
		} else {
			temp := 0.0
			if v, ok := st["temperature"].(float64); ok {
				temp = v
			} else if v, ok := st["temperature"].(int64); ok {
				temp = float64(v)
			}
			seed := 0
			if v, ok := st["seed"].(int64); ok {
				seed = int(v)
			} else if v, ok := st["seed"].(float64); ok {
				seed = int(v)
			}
			jsonMode := false
			if v, ok := st["json_mode"].(bool); ok {
				jsonMode = v
			}
			ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
			resp, d, cerr := llm.Chat(ctx, provider, model, llm.Request{
				Messages:    []llm.Message{{Role: "user", Content: prompt}},
				Temperature: temp,
				Seed:        seed,
				JSONMode:    jsonMode,
			})
			cancel()
			if cerr != nil {
				return "", nil, fmt.Errorf(i18n.T("步骤 %s 调用失败: %v", "step %s call failed: %v"), stepID, cerr)
			}
			text = resp.Text
			rawResp = resp.Raw
			desc = map[string]any{
				"provider": d.Provider, "model": d.Model, "kind": d.Kind,
				"endpoint": d.Endpoint, "temperature": d.Temperature,
				"seed": d.Seed, "json_mode": d.JSONMode,
				"latency_ms": resp.LatencyMS, "usage": resp.Usage,
			}
			level = reproducibilityLevel(d.Provider, d.Temperature, d.Seed)
			fmt.Printf("  [OK]   %s: %s / %s  (%d ms)\n", stepID, d.Provider, d.Model, resp.LatencyMS)
		}

		// 留档：原始响应 + 摘要，供 verify 重放与人工核对
		// live 校验模式（record=false）不写档，避免一次失败的调用覆盖掉好的录制。
		if record {
			writeLLMRecord(recordPath, stepID, promptFile, promptDigest, text, rawResp, desc, level)
		}

		// 供执行体消费的结构化中间表示
		var parsed any
		if err := json.Unmarshal([]byte(text), &parsed); err != nil {
			parsed = map[string]any{"_raw_text": text, "_parse_error": err.Error()}
		}
		writeJSON(filepath.Join(root, runDir, "llm", stepID+".json"), parsed)
		asMap(enriched["llm"])[stepID] = parsed

		descriptors = append(descriptors, map[string]any{
			"step_id": stepID, "provider": asString(desc["provider"]), "model": asString(desc["model"]),
			"prompt_digest": promptDigest, "response_digest": sha256Bytes([]byte(text)),
			"reproducibility": level, "recorded_at": nowISO(), "replayed": replayed,
		})
	}

	enrichedPath := filepath.Join(root, runDir, "input.enriched.json")
	if err := writeJSON(enrichedPath, enriched); err != nil {
		return "", nil, err
	}
	index := map[string]any{
		"schema":       "workagent/llm-index/1",
		"generated_at": nowISO(),
		"steps":        descriptors,
		"level":        overallLevel(descriptors),
	}
	if record {
		writeJSON(filepath.Join(root, llmDir, caseID, "index.json"), index)
	}
	return enrichedPath, descriptors, nil
}

func reproducibilityLevel(provider string, temperature float64, seed int) string {
	p, ok := llm.Resolve(provider)
	localOpenWeights := ok && p.KeyEnv == "" && p.Name != "custom"
	if temperature == 0 && localOpenWeights {
		return "L2-decision-candidate"
	}
	if temperature == 0 {
		return "L1-record-replay"
	}
	return "L0-none"
}

func overallLevel(descs []map[string]any) string {
	if len(descs) == 0 {
		return "L3-deterministic"
	}
	best := "L0-none"
	for _, d := range descs {
		l := asString(d["reproducibility"])
		if l == "L2-decision-candidate" {
			return "L2-decision-candidate"
		}
		if l == "L1-record-replay" {
			best = "L1-record-replay"
		}
	}
	return best
}

// writeLLMRecord 把一次模型调用留档。这是"重放记录而非重放模型"的物质基础。
func writeLLMRecord(path, stepID, promptFile, promptDigest, text, rawResp string, desc map[string]any, level string) {
	writeJSON(path, map[string]any{
		"schema":          "workagent/llm-record/1",
		"step_id":         stepID,
		"prompt_file":     promptFile,
		"prompt_digest":   promptDigest,
		"response_digest": sha256Bytes([]byte(text)),
		"descriptor":      desc,
		"text":            text,
		"raw":             rawResp,
		"recorded_at":     nowISO(),
		"reproducibility": level,
		"honesty_note":    i18n.T("记录的是模型原始响应。重放记录 ≠ 重放模型；L1 只保证裁决过程可复现，不保证模型本身可复现。", "What is recorded is the model's raw response. Replaying a recording is not replaying the model; L1 guarantees the decision process is reproducible, not the model itself."),
	})
}
