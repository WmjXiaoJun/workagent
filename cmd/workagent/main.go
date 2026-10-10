// WorkAgent CLI —— 零依赖的智能体包工具链。
//
// 命令:
//
//	workagent check  [path]                  校验包结构与兼容声明
//	workagent adapt  [path]                  从 agent.yaml 生成适配层
//	workagent attest [path]                  计算 Attestation Closure 摘要
//	workagent run    --input FILE [path]     执行并产出证据包
//	workagent verify [path]                  复现校验：同输入是否产出同结论
//	workagent seal   [path]                  生成全包摘要
//
// 设计原则:
//  1. 零第三方依赖，只用标准库。
//  2. 真源唯一：agent.yaml。adapters/ 是生成物。
//  3. 诚实：没有私钥就只出摘要，不冒充"已签名"。
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

const (
	specVersion = "0.3.0"
	toolVersion = "0.3.0"
	evidenceDir = "out/evidence"
)

var requiredFiles = []string{
	"agent.yaml",
	"compat.json",
	"inputs.schema.json",
	"artifacts/output_schema.json",
	"execution/workflow.yml",
	"execution/runtime.dockerfile",
	"provenance/trace.json",
	"provenance/data_sources.json",
	"provenance/assumptions.json",
	"attest/closure.json",
	"x-workagent/boundary.json",
	"x-workagent/test_suite/unit.json",
	"x-workagent/test_suite/regression.json",
	"x-workagent/test_suite/sensitivity.json",
	"x-workagent/test_suite/stress.json",
}

// ---------------------------------------------------------------- helpers

func ok(msg string)   { fmt.Println("  [OK]   " + msg) }
func bad(msg string)  { fmt.Println("  [FAIL] " + msg) }
func info(msg string) { fmt.Println("  [--]   " + msg) }

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func sha256Bytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func sha256File(p string) (string, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	return sha256Bytes(b), nil
}

// canonicalJSON 规范化 JSON：键排序、无多余空白、不转义 HTML。
// 结论摘要一律基于它计算，保证同一对象永远得到同一摘要。
func canonicalJSON(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

func readJSON(p string) (any, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	dec.UseNumber() // 保留数字原始字面量，避免浮点格式漂移
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return v, nil
}

func writeJSON(p string, v any) error {
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, append(b, '\n'), 0o644)
}

func nowISO() string { return time.Now().Format("2006-01-02T15:04:05-07:00") }

func toolchainVersion() string { return "go " + runtime.Version() }

func platformName() string { return runtime.GOOS + "/" + runtime.GOARCH }

func asMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

func asString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case nil:
		return ""
	default:
		return fmt.Sprint(t)
	}
}

func asList(v any) []any {
	if l, ok := v.([]any); ok {
		return l
	}
	return nil
}

func resolveRoot(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return abs
}

func short(v any) string {
	s := asString(v)
	if s == "" {
		return "n/a"
	}
	if len(s) > 16 {
		return s[:16]
	}
	return s
}

func mustManifest(root string) map[string]any {
	p := filepath.Join(root, "agent.yaml")
	b, err := os.ReadFile(p)
	if err != nil {
		fmt.Fprintln(os.Stderr, "找不到 agent.yaml —— 请确认这是 WorkAgent 包根目录")
		os.Exit(2)
	}
	m, isMap := parseYAML(string(b)).(map[string]any)
	if !isMap {
		fmt.Fprintln(os.Stderr, "agent.yaml 解析失败")
		os.Exit(2)
	}
	return m
}

func executorCommand(m map[string]any) []string {
	execNode := asMap(asMap(m["execution"])["executor"])
	var out []string
	for _, c := range asList(execNode["command"]) {
		out = append(out, asString(c))
	}
	return out
}

// resolveExecCommand 解决 python / python3 的跨平台差异：
// 声明的是 python 但系统只有 python3（或反之）时，自动落到可用的那个。
func resolveExecCommand(cmd []string) []string {
	if len(cmd) == 0 {
		return cmd
	}
	if _, err := exec.LookPath(cmd[0]); err == nil {
		return cmd
	}
	alt := ""
	switch cmd[0] {
	case "python":
		alt = "python3"
	case "python3":
		alt = "python"
	}
	if alt != "" {
		if _, err := exec.LookPath(alt); err == nil {
			out := append([]string{}, cmd...)
			out[0] = alt
			return out
		}
	}
	return cmd
}

// ---------------------------------------------------------------- check

func cmdCheck(root string) int {
	fmt.Println("WorkAgent check —— " + root)
	failures := 0

	fmt.Println("\n[1/4] 必需文件")
	for _, rel := range requiredFiles {
		if fileExists(filepath.Join(root, filepath.FromSlash(rel))) {
			ok(rel)
		} else {
			bad(rel + " 缺失")
			failures++
		}
	}

	fmt.Println("\n[2/4] JSON 可解析性")
	for _, rel := range requiredFiles {
		if !strings.HasSuffix(rel, ".json") {
			continue
		}
		if _, err := readJSON(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			bad(rel + " -> " + err.Error())
			failures++
		} else {
			ok(rel)
		}
	}

	fmt.Println("\n[3/4] 兼容声明")
	compatRaw, err := readJSON(filepath.Join(root, "compat.json"))
	if err != nil {
		bad("compat.json -> " + err.Error())
		failures++
	} else {
		compat := asMap(compatRaw)
		matrix := asList(compat["capability_matrix"])
		var lost []string
		for _, c := range matrix {
			if asString(asMap(c)["on_legacy"]) == "lost" {
				lost = append(lost, asString(asMap(c)["capability"]))
			}
		}
		ok(fmt.Sprintf("compat.json 已声明 %d 项能力", len(matrix)))
		if len(lost) > 0 {
			info("旧平台将丢失: " + strings.Join(lost, ", "))
		} else {
			info("旧平台将丢失: 无")
		}
		if len(asMap(compat["degradation"])) == 0 {
			bad("缺少 degradation 段：无法说明降级行为")
			failures++
		} else {
			ok("已显式声明降级行为")
		}
	}

	fmt.Println("\n[4/4] 边界与测试套件")
	boundaryRaw, err := readJSON(filepath.Join(root, "x-workagent", "boundary.json"))
	if err != nil {
		bad("boundary.json -> " + err.Error())
		failures++
	} else if asString(asMap(boundaryRaw)["stop_condition"]) == "" {
		bad("boundary.json 缺 stop_condition")
		failures++
	} else {
		ok("boundary.json 含 stop_condition")
	}
	suites, _ := filepath.Glob(filepath.Join(root, "x-workagent", "test_suite", "*.json"))
	names := make([]string, 0, len(suites))
	for _, s := range suites {
		names = append(names, strings.TrimSuffix(filepath.Base(s), ".json"))
	}
	sort.Strings(names)
	ok(fmt.Sprintf("测试套件 %d 个: %s", len(names), strings.Join(names, ", ")))

	fmt.Println("")
	if failures > 0 {
		fmt.Printf("结果: 不通过，共 %d 项问题\n", failures)
		return 1
	}
	fmt.Println("结果: 通过。本包结构完整、兼容声明齐备。")
	return 0
}

// ---------------------------------------------------------------- adapt

func cmdAdapt(root string) int {
	m := mustManifest(root)
	name := asString(m["name"])
	if name == "" {
		parts := strings.Split(asString(m["id"]), "/")
		name = parts[len(parts)-1]
	}
	descEn := asString(m["summary_en"])
	if descEn == "" {
		descEn = asString(m["summary"])
	}
	displayEn := asString(m["display_name_en"])
	if displayEn == "" {
		displayEn = name
	}

	plugin := map[string]any{
		"name":        name,
		"version":     asString(m["version"]),
		"description": descEn,
		"author":      asString(asMap(m["author"])["name"]),
		"license":     asString(m["license"]),
		"homepage":    "",
		"repository":  "",
		"components": map[string]any{
			"skills":     []any{},
			"mcpServers": map[string]any{},
		},
		"x-workagent": map[string]any{
			"manifest": "agent.yaml",
			"compat":   "compat.json",
			"boundary": "x-workagent/boundary.json",
			"note":     "生成物。真源为根目录 agent.yaml。",
		},
	}
	if err := writeJSON(filepath.Join(root, "adapters", "plugin.json"), plugin); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	card := map[string]any{
		"name":               displayEn,
		"description":        descEn,
		"url":                "",
		"version":            asString(m["version"]),
		"protocolVersion":    "1.0",
		"preferredTransport": "JSONRPC",
		"defaultInputModes":  []any{"application/json"},
		"defaultOutputModes": []any{"application/json", "text/markdown"},
		"capabilities": map[string]any{
			"streaming":              false,
			"pushNotifications":      false,
			"stateTransitionHistory": true,
		},
		"skills": []any{},
		"x-workagent": map[string]any{
			"manifest": "agent.yaml",
			"evidence": evidenceDir,
			"note":     "生成物。真源为根目录 agent.yaml。",
		},
	}
	if err := writeJSON(filepath.Join(root, "adapters", "agent-card.json"), card); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	fmt.Println("adapt —— 已生成适配层")
	ok("adapters/plugin.json      -> Agent Plugins 1.0")
	ok("adapters/agent-card.json  -> A2A Agent Card")
	info("真源未改动：agent.yaml")
	return 0
}

// ---------------------------------------------------------------- attest

func cmdAttest(root string) int {
	m := mustManifest(root)
	cmd := executorCommand(m)
	var execDigest any
	if len(cmd) > 1 {
		if h, err := sha256File(filepath.Join(root, filepath.FromSlash(cmd[1]))); err == nil {
			execDigest = h
		}
	}
	schemaDigest, _ := sha256File(filepath.Join(root, "inputs.schema.json"))
	wfDigest, _ := sha256File(filepath.Join(root, "execution", "workflow.yml"))
	manifestDigest, _ := sha256File(filepath.Join(root, "agent.yaml"))

	closure := map[string]any{
		"schema":       "workagent/attest/closure/1",
		"generated_at": nowISO(),
		"principle":    "摘要必须覆盖推理函数的完整计算闭包。任何落在被证明集合之外的组件，都是一条伪造通道。",
		"closure": map[string]any{
			"executor_digest":      execDigest,
			"executor_command":     strings.Join(cmd, " "),
			"inputs_schema_digest": schemaDigest,
			"workflow_digest":      wfDigest,
			"manifest_digest":      manifestDigest,
			"toolchain":            toolchainVersion(),
			"platform":             platformName(),
			"numeric_env": map[string]any{
				"PYTHONHASHSEED": "0",
				"LC_ALL":         "C.UTF-8",
				"TZ":             "UTC",
			},
			"model_weights_digest": nil,
			"tokenizer_digest":     nil,
			"chat_template_digest": nil,
			"engine_build_digest":  nil,
			"decoding_config":      nil,
		},
		"llm_used": len(asList(asMap(m["execution"])["llm_steps"])) > 0,
		"note":     "确定性执行体不涉及模型闭包；一旦启用 LLM 步骤，model/tokenizer/chat_template/engine/decoding 字段必须填写，否则不能声称结论可复现。执行体自身的运行时版本（如 Python/Node）由执行体作者负责登记。",
	}
	if err := writeJSON(filepath.Join(root, "attest", "closure.json"), closure); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	fmt.Println("attest —— 计算闭包摘要已写入 attest/closure.json")
	ok("executor_digest      " + short(execDigest))
	ok("inputs_schema_digest " + short(schemaDigest))
	ok("workflow_digest      " + short(wfDigest))
	ok("manifest_digest      " + short(manifestDigest))
	ok("toolchain            " + toolchainVersion())
	info(fmt.Sprintf("llm_used = %v", closure["llm_used"]))
	return 0
}

// ---------------------------------------------------------------- run

func runExecutor(root string, m map[string]any, inputPath string) (any, []string, error) {
	cmd := resolveExecCommand(executorCommand(m))
	if len(cmd) == 0 {
		return nil, nil, fmt.Errorf("agent.yaml 中未声明 execution.executor.command")
	}
	args := append(append([]string{}, cmd[1:]...), inputPath)
	c := exec.Command(cmd[0], args...)
	c.Dir = root
	var stdout, stderr bytes.Buffer
	c.Stdout = &stdout
	c.Stderr = &stderr
	if err := c.Run(); err != nil {
		return nil, cmd, fmt.Errorf("执行体失败 (%v): %s", err, strings.TrimSpace(stderr.String()))
	}
	dec := json.NewDecoder(bytes.NewReader(stdout.Bytes()))
	dec.UseNumber()
	var payload any
	if err := dec.Decode(&payload); err != nil {
		return nil, cmd, fmt.Errorf("执行体输出不是合法 JSON: %v", err)
	}
	if pm, isMap := payload.(map[string]any); isMap {
		if out, exists := pm["output"]; exists {
			return out, cmd, nil
		}
	}
	return payload, cmd, nil
}

func cmdRun(root, inputArg string, replay bool, providerOverride, modelOverride string) int {
	m := mustManifest(root)
	if inputArg == "" {
		fmt.Fprintln(os.Stderr, "run 需要 --input 参数")
		return 2
	}
	inputPath := inputArg
	if !filepath.IsAbs(inputPath) {
		inputPath = filepath.Join(root, filepath.FromSlash(inputArg))
	}
	if !fileExists(inputPath) {
		fmt.Fprintln(os.Stderr, "找不到输入文件: "+inputPath)
		return 2
	}

	inputObj, err := readJSON(inputPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "输入不是合法 JSON: "+err.Error())
		return 2
	}
	inCanon, _ := canonicalJSON(inputObj)
	inputDigest := sha256Bytes(inCanon)

	// LLM 步骤（可选）：在确定性执行体之前完成"抽取"，把非结构化输入变成结构化中间表示。
	// 执行体拿到的始终是确定性数据，因此裁决过程可复现。
	execInput := inputPath
	var llmDescriptors []map[string]any
	if len(llmStepsOf(m)) > 0 {
		enriched, descs, lerr := runLLMSteps(root, m, inputObj, replay, providerOverride, modelOverride, true)
		if lerr != nil {
			fmt.Fprintln(os.Stderr, lerr.Error())
			return 1
		}
		llmDescriptors = descs
		if enriched != "" {
			execInput = enriched
		}
	}

	outputObj, cmd, err := runExecutor(root, m, execInput)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}
	outCanon, _ := canonicalJSON(outputObj)
	outputDigest := sha256Bytes(outCanon)

	evidenceID := sha256Bytes([]byte(inputDigest + outputDigest + asString(m["version"])))[:16]
	evDir := filepath.Join(root, evidenceDir, evidenceID)
	if err := os.MkdirAll(evDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	writeJSON(filepath.Join(evDir, "input.json"), inputObj)
	writeJSON(filepath.Join(evDir, "output.json"), outputObj)

	tpl := "# 结论摘要\n\n<conclusion>\n"
	if b, rerr := os.ReadFile(filepath.Join(root, "artifacts", "summary.template.md")); rerr == nil {
		tpl = string(b)
	}
	outMap := asMap(outputObj)
	conf := asMap(outMap["confidence"])
	var riskLines []string
	for _, r := range asList(outMap["risks"]) {
		riskLines = append(riskLines, "- "+asString(r))
	}
	riskText := "- （无）"
	if len(riskLines) > 0 {
		riskText = strings.Join(riskLines, "\n")
	}
	execDigest := "n/a"
	if len(cmd) > 1 {
		if h, herr := sha256File(filepath.Join(root, filepath.FromSlash(cmd[1]))); herr == nil {
			execDigest = h[:16]
		}
	}
	repl := map[string]string{
		"<project>":          asString(outMap["project"]),
		"<conclusion>":       asString(outMap["conclusion"]),
		"<currency>":         asString(outMap["currency"]),
		"<total_price>":      asString(outMap["total_price"]),
		"<confidence.level>": asString(conf["level"]),
		"<confidence.basis>": asString(conf["basis"]),
		"<risks>":            riskText,
		"<generated_at>":     nowISO(),
		"<executor_digest>":  execDigest,
		"<output_digest>":    outputDigest[:16],
		"<evidence_id>":      evidenceID,
	}
	for k, v := range repl {
		tpl = strings.ReplaceAll(tpl, k, v)
	}
	os.WriteFile(filepath.Join(evDir, "summary.md"), []byte(tpl), 0o644)
	os.WriteFile(filepath.Join(root, "artifacts", "summary.md"), []byte(tpl), 0o644)

	traceRaw, err := readJSON(filepath.Join(root, "provenance", "trace.json"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	trace := asMap(traceRaw)
	steps := asList(trace["steps"])
	trace["evidence_id"] = evidenceID
	trace["steps"] = append(steps, map[string]any{
		"seq":           len(steps) + 1,
		"at":            nowISO(),
		"step":          "run",
		"executor":      strings.Join(cmd, " "),
		"input_file":    inputPath,
		"input_digest":  inputDigest,
		"output_digest": outputDigest,
		"evidence_id":   evidenceID,
	})
	writeJSON(filepath.Join(root, "provenance", "trace.json"), trace)

	writeJSON(filepath.Join(evDir, "evidence.json"), map[string]any{
		"schema":          "workagent/evidence/1",
		"evidence_id":     evidenceID,
		"generated_at":    nowISO(),
		"spec_version":    specVersion,
		"tool_version":    toolVersion,
		"executor":        strings.Join(cmd, " "),
		"input_digest":    inputDigest,
		"output_digest":   outputDigest,
		"llm_steps":       llmDescriptors,
		"reproducibility": overallLevel(llmDescriptors),
		"output":          outputObj,
	})

	fmt.Println("run —— 执行完成")
	ok("证据包编号 " + evidenceID)
	ok("结论: " + asString(outMap["conclusion"]))
	ok("输出摘要 " + outputDigest[:16])
	ok("复现等级 " + overallLevel(llmDescriptors))
	if len(llmDescriptors) > 0 {
		info("证据包含模型响应留档；verify 默认重放记录而非重放模型")
	}
	info("证据目录 " + evDir)
	info("摘要文件 " + filepath.Join(root, "artifacts", "summary.md"))
	return 0
}

// ---------------------------------------------------------------- verify

func cmdVerify(root string, live bool) int {
	m := mustManifest(root)
	traceRaw, err := readJSON(filepath.Join(root, "provenance", "trace.json"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	var last map[string]any
	for _, s := range asList(asMap(traceRaw)["steps"]) {
		sm := asMap(s)
		if asString(sm["step"]) == "run" {
			last = sm
		}
	}
	if last == nil {
		fmt.Println("verify —— 没有可复现的记录，请先运行 workagent run")
		return 1
	}
	fmt.Println("verify —— 复现校验，证据包 " + asString(last["evidence_id"]))

	inputFile := asString(last["input_file"])
	if !fileExists(inputFile) {
		inputFile = filepath.Join(root, evidenceDir, asString(last["evidence_id"]), "input.json")
	}
	if !fileExists(inputFile) {
		bad("找不到原始输入，无法复现")
		return 1
	}

	// 含 LLM 步骤时：默认重放已录制的模型响应（离线、可复现）；
	// --live 则重新真实调用模型，用来**测量**模型本身是否可复现。
	execInput := inputFile
	liveDiffers := false
	if len(llmStepsOf(m)) > 0 {
		inObj, ierr := readJSON(inputFile)
		if ierr != nil {
			fmt.Fprintln(os.Stderr, ierr.Error())
			return 1
		}
		if live {
			// 关键：必须**先**读出旧录制的摘要，再重新调用。
			// 否则拿刚被覆盖的档去比对，永远相等、永远 PASS —— 那样的测试是摆设。
			oldDigests := map[string]string{}
			for _, st := range llmStepsOf(m) {
				sid := asString(asMap(st)["id"])
				if rec, rerr := readJSON(filepath.Join(root, llmDir, sid+".json")); rerr == nil {
					oldDigests[sid] = asString(asMap(rec)["response_digest"])
				}
			}
			enriched, descs, lerr := runLLMSteps(root, m, inObj, false, "", "", false)
			if lerr != nil {
				fmt.Fprintln(os.Stderr, "live 调用失败: "+lerr.Error())
				return 1
			}
			if enriched != "" {
				execInput = enriched
			}
			for _, d := range descs {
				sid := asString(d["step_id"])
				old := oldDigests[sid]
				neu := asString(d["response_digest"])
				if old == "" {
					fmt.Println("  [!!]   " + sid + " 没有历史录制，无法比较模型是否复现")
					continue
				}
				if old == neu {
					fmt.Println("  [OK]   " + sid + " 模型本次响应与录制**逐字节一致**（" + short(neu) + "）")
				} else {
					liveDiffers = true
					fmt.Println("  [!!]   " + sid + " 模型本次响应与录制不同")
					fmt.Println("         录制 " + short(old) + "  ≠  本次 " + short(neu))
					fmt.Println("         —— 该模型在 temperature=0/seed 固定下**仍然不可复现**")
				}
			}
		} else {
			fmt.Println("  [--]   LLM 步骤：重放已录制响应（未联网）")
			enriched, _, lerr := runLLMSteps(root, m, inObj, true, "", "", false)
			if lerr != nil {
				fmt.Fprintln(os.Stderr, lerr.Error())
				return 1
			}
			if enriched != "" {
				execInput = enriched
			}
		}
	}

	outputObj, _, err := runExecutor(root, m, execInput)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}
	canon, _ := canonicalJSON(outputObj)
	replay := sha256Bytes(canon)
	expected := asString(last["output_digest"])
	if live && liveDiffers {
		fmt.Println("结果: FAIL（live）—— 模型本次未复现，因此该结论不可复现。请用 replay 模式交付。")
		return 1
	}

	fmt.Println("  期望摘要 " + expected)
	fmt.Println("  重放摘要 " + replay)
	if replay == expected {
		fmt.Println("结果: PASS —— 同输入产出同结论，可复现。")
		return 0
	}
	fmt.Println("结果: FAIL —— 结论不可复现，禁止对外交付该结论。")
	return 1
}

// ---------------------------------------------------------------- seal

func cmdSeal(root string) int {
	skipDirs := map[string]bool{
		".git": true, "__pycache__": true, "out": true, "adapters": true,
		".venv": true, "dist": true, "build": true, "bin": true,
	}
	var entries []map[string]any
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		if d.IsDir() {
			if p != root && skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return rerr
		}
		rel = filepath.ToSlash(rel)
		if rel == "signature/agent.sig" {
			return nil
		}
		h, herr := sha256File(p)
		if herr != nil {
			return herr
		}
		entries = append(entries, map[string]any{"path": rel, "sha256": h})
		return nil
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	sort.Slice(entries, func(i, j int) bool {
		return asString(entries[i]["path"]) < asString(entries[j]["path"])
	})
	parts := make([]string, 0, len(entries))
	for _, e := range entries {
		parts = append(parts, asString(e["path"])+":"+asString(e["sha256"]))
	}
	packageDigest := sha256Bytes([]byte(strings.Join(parts, "\n")))

	sig := map[string]any{
		"schema":         "workagent/signature/1",
		"generated_at":   nowISO(),
		"spec_version":   specVersion,
		"tool_version":   toolVersion,
		"scheme":         "digest-only",
		"signer":         nil,
		"public_key":     nil,
		"package_digest": packageDigest,
		"file_count":     len(entries),
		"files":          entries,
		"honesty_note":   "digest-only 只证明『内容自生成后未变』，不证明『谁签的』。如需不可否认性，请用 GPG/Ed25519 对 package_digest 另行签名，并把公钥放入 signature/。",
	}
	if err := writeJSON(filepath.Join(root, "signature", "agent.sig"), sig); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	fmt.Println("seal —— 全包摘要已生成")
	ok("package_digest " + packageDigest)
	ok(fmt.Sprintf("文件数 %d", len(entries)))
	info("scheme = digest-only（未冒充密码学签名）")
	return 0
}

// ---------------------------------------------------------------- main

func usage() {
	fmt.Println("WorkAgent 包工具链 v" + toolVersion + "\n\n" +
		"用法:\n" +
		"  workagent check  [path]                 校验包结构与兼容声明\n" +
		"  workagent adapt  [path]                 从 agent.yaml 生成适配层\n" +
		"  workagent attest [path]                 计算 Attestation Closure 摘要\n" +
		"  workagent run    --input FILE [path]    执行并产出证据包\n" +
		"  workagent run    --input FILE --replay  使用已录制的模型响应（离线）\n" +
		"  workagent models                        列出内置模型服务商与可用状态\n" +
		"  workagent verify [path]                 复现校验：同输入是否产出同结论\n" +
		"  workagent verify [path] --live          重新真实调用模型，测量其是否可复现\n" +
		"  workagent seal   [path]                 生成全包摘要\n" +
		"\npath 默认为当前目录。")
}

func pathArg(args []string) string {
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			return resolveRoot(a)
		}
	}
	return resolveRoot(".")
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(0)
	}
	switch os.Args[1] {
	case "-v", "--version", "version":
		fmt.Println("workagent " + toolVersion)
		return
	}
	var code int
	args := os.Args[2:]
	switch os.Args[1] {
	case "check":
		code = cmdCheck(pathArg(args))
	case "adapt":
		code = cmdAdapt(pathArg(args))
	case "attest":
		code = cmdAttest(pathArg(args))
	case "models":
		code = cmdModels()
	case "run":
		fs := flag.NewFlagSet("run", flag.ContinueOnError)
		input := fs.String("input", "", "输入 JSON 路径")
		replayFlag := fs.Bool("replay", false, "使用已录制的模型响应（离线，可复现）")
		provider := fs.String("provider", "", "覆盖 manifest 中的 LLM provider")
		model := fs.String("model", "", "覆盖 manifest 中的 LLM model")
		if err := fs.Parse(args); err != nil {
			os.Exit(2)
		}
		code = cmdRun(pathArg(fs.Args()), *input, *replayFlag, *provider, *model)
	case "verify":
		fsv := flag.NewFlagSet("verify", flag.ContinueOnError)
		live := fsv.Bool("live", false, "重新真实调用模型，测量模型本身是否可复现")
		if err := fsv.Parse(args); err != nil {
			os.Exit(2)
		}
		code = cmdVerify(pathArg(fsv.Args()), *live)
	case "seal":
		code = cmdSeal(pathArg(args))
	default:
		usage()
		code = 1
	}
	os.Exit(code)
}
