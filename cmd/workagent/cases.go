package main

// 多场景（cases）：一个 WorkAgent 包可以承载多个可验证业务场景，
// 共用同一套基础设施（兼容声明、溯源、证据包、复现校验）。
//
// 向后兼容：若 agent.yaml 没有 execution.cases，则用 execution.executor 合成一个隐式场景。

import (
	"fmt"
	"path/filepath"

	"github.com/WmjXiaoJun/workagent/internal/i18n"
)

// Case 是一个可验证的业务场景。
type Case struct {
	ID            string
	Title         string
	TitleEn       string
	Shape         string
	Executor      []string
	LLMSteps      []any
	Boundary      string
	SampleInput   string
	SampleNatural string
	Note          string
}

// DisplayTitle 返回当前语言下的场景标题。
// 英文缺失时回落到中文 —— 宁可显示中文，也不要显示空白。
func (c Case) DisplayTitle() string {
	if i18n.IsEn() && c.TitleEn != "" {
		return c.TitleEn
	}
	return c.Title
}

func casesOf(m map[string]any) []Case {
	execNode := asMap(m["execution"])
	var out []Case
	for _, r := range asList(execNode["cases"]) {
		cm := asMap(r)
		c := Case{
			ID:            asString(cm["id"]),
			Title:         asString(cm["title"]),
			TitleEn:       asString(cm["title_en"]),
			Shape:         asString(cm["shape"]),
			Boundary:      asString(cm["boundary"]),
			SampleInput:   asString(cm["sample_input"]),
			SampleNatural: asString(cm["sample_natural"]),
			Note:          asString(cm["note"]),
		}
		for _, e := range asList(cm["executor"]) {
			c.Executor = append(c.Executor, asString(e))
		}
		c.LLMSteps = asList(cm["llm_steps"])
		if c.ID != "" {
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		var cmdv []string
		for _, e := range asList(asMap(execNode["executor"])["command"]) {
			cmdv = append(cmdv, asString(e))
		}
		if len(cmdv) > 0 {
			out = append(out, Case{
				ID:       "default",
				Title:    "默认场景",
				Executor: cmdv,
				LLMSteps: asList(execNode["llm_steps"]),
				Boundary: "x-workagent/boundary.json",
			})
		}
	}
	return out
}

func findCase(m map[string]any, id string) (Case, bool) {
	all := casesOf(m)
	if id == "" {
		if def := asString(asMap(m["execution"])["default_case"]); def != "" {
			for _, c := range all {
				if c.ID == def {
					return c, true
				}
			}
		}
		if len(all) > 0 {
			return all[0], true
		}
		return Case{}, false
	}
	for _, c := range all {
		if c.ID == id {
			return c, true
		}
	}
	return Case{}, false
}

func cmdCases(root string) int {
	m := mustManifest(root)
	all := casesOf(m)
	fmt.Println(i18n.T("WorkAgent cases —— 按交付形态分组", "WorkAgent cases — grouped by delivery form"))
	fmt.Println("")
	// 形态顺序固定：测算 → 核验 → 抽取 → 排程 → 推演
	order := []string{"测算型", "核验型", "抽取型", "排程型", "推演型"}
	grouped := map[string][]Case{}
	seen := map[string]bool{}
	var extra []string
	for _, c := range all {
		s := c.Shape
		if s == "" {
			s = "未分类"
		}
		if !seen[s] {
			seen[s] = true
			known := false
			for _, o := range order {
				if o == s {
					known = true
					break
				}
			}
			if !known {
				extra = append(extra, s)
			}
		}
		grouped[s] = append(grouped[s], c)
	}
	for _, s := range append(append([]string{}, order...), extra...) {
		cs := grouped[s]
		if len(cs) == 0 {
			continue
		}
		fmt.Println(i18n.Tf("【%s】%d 个", "[%s] %d", i18n.Shape(s), len(cs)))
		for _, c := range cs {
			rep := "L3"
			if len(c.LLMSteps) > 0 {
				rep = "L2/L1"
			}
			fmt.Printf("  %-16s %-8s %s\n", c.ID, rep, c.DisplayTitle())
		}
		fmt.Println("")
	}
	fmt.Println("")
	fmt.Println(i18n.T("用法:", "Usage:"))
	fmt.Println("  workagent run --case <CASE> --input <file> [--replay] [path]")
	fmt.Println(i18n.T("  workagent cases                              列出全部场景",
		"  workagent cases                              list all scenarios"))
	fmt.Println("")
	fmt.Println(i18n.T("复现等级: L3 纯确定性 / L2 本地开权重模型 / L1 云端模型（靠录制重放）",
		"Reproducibility: L3 deterministic / L2 local open-weights / L1 cloud (replay from recording)"))
	fmt.Println(i18n.T("形态说明: 测算型 · 核验型 · 抽取型 · 排程型 · 推演型 —— 见 docs/VERTICALS.md",
		"Delivery forms: Estimation · Verification · Extraction · Scheduling · Simulation — see docs/VERTICALS.md"))
	return 0
}

func caseSample(root string, c Case, natural bool) string {
	p := c.SampleInput
	if natural && c.SampleNatural != "" {
		p = c.SampleNatural
	}
	if p == "" {
		return ""
	}
	return filepath.Join(root, filepath.FromSlash(p))
}

// cmdRunAll 用各场景自带的样例依次运行（样例输入与自然语言输入都跑）。
// 这样新增场景时 CI 不需要改动 —— 少一处会忘记同步的地方。
func cmdRunAll(root string, replay bool) int {
	m := mustManifest(root)
	all := casesOf(m)
	if len(all) == 0 {
		fmt.Println("run --all —— 本包没有场景")
		return 1
	}
	mode := "真实调用模型"
	if replay {
		mode = "重放已录制响应（离线）"
	}
	fmt.Printf("run --all —— 共 %d 个场景，模型步骤：%s\n\n", len(all), mode)
	failures, runs := 0, 0
	for _, c := range all {
		var inputs []string
		if c.SampleInput != "" {
			inputs = append(inputs, c.SampleInput)
		}
		if c.SampleNatural != "" && c.SampleNatural != c.SampleInput {
			inputs = append(inputs, c.SampleNatural)
		}
		if len(inputs) == 0 {
			fmt.Println("  [FAIL] " + c.ID + " 未声明 sample_input")
			failures++
			continue
		}
		for _, in := range inputs {
			runs++
			if code := cmdRun(root, c.ID, in, replay, "", ""); code != 0 {
				failures++
			}
		}
	}
	fmt.Println("")
	if failures > 0 {
		fmt.Printf("结果: %d 个场景运行失败（共执行 %d 次）\n", failures, runs)
		return 1
	}
	fmt.Printf("结果: 全部 %d 个场景运行完成（共执行 %d 次）\n", len(all), runs)
	return 0
}
