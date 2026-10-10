// WorkAgent CLI —— 零依赖的智能体包工具链。
//
// 命令:
//
//	workagent check  [path]                  校验包结构与兼容声明
//	workagent adapt  [path]                  从 agent.yaml 生成适配层
//	workagent attest [path]                  计算 Attestation Closure 摘要
//	workagent run    --input FILE [path]     执行并产出证据包
//	workagent verify [path]                  复现校验：同输入是否产出同结论
//	workagent record --case ID [--input F]   跑一次并把结论+证据公开留档
//	workagent mcp    [path]                  以 MCP server 方式暴露本包（stdio）
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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/WmjXiaoJun/workagent/internal/i18n"
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

// relToRoot 把包内文件记成相对路径 —— trace 是跨机器复现的依据，
// 记绝对路径会让它在另一台机器上直接失效（真被 CI 抓到过）。
func relToRoot(root, p string) string {
	r, err := filepath.Rel(root, p)
	if err != nil || strings.HasPrefix(r, "..") {
		return p
	}
	return filepath.ToSlash(r)
}

// resolveInputPath 兼容相对与绝对两种记录。
func resolveInputPath(root, p string) string {
	if p == "" || filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(root, filepath.FromSlash(p))
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
		fmt.Fprintln(os.Stderr, i18n.T("找不到 agent.yaml —— 请确认这是 WorkAgent 包根目录", "agent.yaml not found — confirm this is a WorkAgent package root"))
		os.Exit(2)
	}
	m, isMap := parseYAML(string(b)).(map[string]any)
	if !isMap {
		fmt.Fprintln(os.Stderr, i18n.T("agent.yaml 解析失败", "failed to parse agent.yaml"))
		os.Exit(2)
	}
	return m
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

	fmt.Println(i18n.T("\n[1/4] 必需文件", "\n[1/4] Required files"))
	for _, rel := range requiredFiles {
		if fileExists(filepath.Join(root, filepath.FromSlash(rel))) {
			ok(rel)
		} else {
			bad(rel + i18n.T(" 缺失", " missing"))
			failures++
		}
	}

	fmt.Println(i18n.T("\n[2/4] JSON 可解析性", "\n[2/4] JSON parseability"))
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

	fmt.Println(i18n.T("\n[3/4] 兼容声明", "\n[3/4] Compatibility declaration"))
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
		ok(i18n.Tf("compat.json 已声明 %d 项能力", "compat.json declares %d capabilities", len(matrix)))
		if len(lost) > 0 {
			info(i18n.T("旧平台将丢失: ", "Legacy platforms will lose: ") + strings.Join(lost, ", "))
		} else {
			info(i18n.T("旧平台将丢失: 无", "Legacy platforms will lose: nothing"))
		}
		if len(asMap(compat["degradation"])) == 0 {
			bad(i18n.T("缺少 degradation 段：无法说明降级行为", "missing the degradation section: cannot state what legacy platforms lose"))
			failures++
		} else {
			ok(i18n.T("已显式声明降级行为", "degradation behaviour is declared explicitly"))
		}
	}

	fmt.Println(i18n.T("\n[4/5] 边界与测试套件", "\n[4/5] Boundary and test suites"))
	boundaryRaw, err := readJSON(filepath.Join(root, "x-workagent", "boundary.json"))
	if err != nil {
		bad("boundary.json -> " + err.Error())
		failures++
	} else if asString(asMap(boundaryRaw)["stop_condition"]) == "" {
		bad(i18n.T("boundary.json 缺 stop_condition", "boundary.json is missing stop_condition"))
		failures++
	} else {
		ok(i18n.T("boundary.json 含 stop_condition", "boundary.json declares stop_condition"))
	}
	suites, _ := filepath.Glob(filepath.Join(root, "x-workagent", "test_suite", "*.json"))
	names := make([]string, 0, len(suites))
	for _, s := range suites {
		names = append(names, strings.TrimSuffix(filepath.Base(s), ".json"))
	}
	sort.Strings(names)
	ok(i18n.Tf("测试套件 %d 个: %s", "Test suites (%d): %s", len(names), strings.Join(names, ", ")))

	fmt.Println(i18n.T("\n[5/5] 场景（cases）", "\n[5/5] Scenarios (cases)"))
	for _, cc := range casesOf(mustManifest(root)) {
		var missing []string
		if len(cc.Executor) > 1 && !fileExists(filepath.Join(root, filepath.FromSlash(cc.Executor[1]))) {
			missing = append(missing, cc.Executor[1])
		}
		if cc.Boundary != "" && !fileExists(filepath.Join(root, filepath.FromSlash(cc.Boundary))) {
			missing = append(missing, cc.Boundary)
		}
		if cc.SampleInput != "" && !fileExists(filepath.Join(root, filepath.FromSlash(cc.SampleInput))) {
			missing = append(missing, cc.SampleInput)
		}
		if cc.SampleNatural != "" && !fileExists(filepath.Join(root, filepath.FromSlash(cc.SampleNatural))) {
			missing = append(missing, cc.SampleNatural)
		}
		for _, st := range cc.LLMSteps {
			if pf := asString(asMap(st)["prompt"]); pf != "" && !fileExists(filepath.Join(root, filepath.FromSlash(pf))) {
				missing = append(missing, pf)
			}
		}
		if len(missing) > 0 {
			bad(cc.ID + i18n.T(" 缺少: ", " missing: ") + strings.Join(missing, ", "))
			failures++
		} else {
			ok(cc.ID + "  (" + i18n.Shape(cc.Shape) + ")")
		}
	}

	fmt.Println("")
	if failures > 0 {
		fmt.Print(i18n.Tf("结果: 不通过，共 %d 项问题\n", "Result: FAILED, %d problem(s)\n", failures))
		return 1
	}
	fmt.Println(i18n.T("结果: 通过。本包结构完整、兼容声明齐备。", "Result: PASS. Package structure complete and compatibility declaration present."))
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
			"note":     i18n.T("生成物。真源为根目录 agent.yaml。", "Generated artifact. The source of truth is agent.yaml at the package root."),
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
			"note":     i18n.T("生成物。真源为根目录 agent.yaml。", "Generated artifact. The source of truth is agent.yaml at the package root."),
		},
	}
	if err := writeJSON(filepath.Join(root, "adapters", "agent-card.json"), card); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	fmt.Println(i18n.T("adapt —— 已生成适配层", "adapt — adapter layer generated"))
	ok("adapters/plugin.json      -> Agent Plugins 1.0")
	ok("adapters/agent-card.json  -> A2A Agent Card")
	info(i18n.T("真源未改动：agent.yaml", "Source of truth untouched: agent.yaml"))
	return 0
}

// ---------------------------------------------------------------- attest

func cmdAttest(root string) int {
	m := mustManifest(root)
	all := casesOf(m)
	def, _ := findCase(m, "")
	cmd := def.Executor
	var execDigest any
	if len(cmd) > 1 {
		if h, err := sha256File(filepath.Join(root, filepath.FromSlash(cmd[1]))); err == nil {
			execDigest = h
		}
	}
	// 逐场景记录执行体摘要：多场景包里，每个场景都是独立的计算闭包
	caseExec := map[string]any{}
	llmUsed := false
	for _, cc := range all {
		entry := map[string]any{"command": strings.Join(cc.Executor, " "), "llm_steps": len(cc.LLMSteps)}
		if len(cc.Executor) > 1 {
			if h, err := sha256File(filepath.Join(root, filepath.FromSlash(cc.Executor[1]))); err == nil {
				entry["digest"] = h
			}
		}
		if len(cc.LLMSteps) > 0 {
			llmUsed = true
		}
		caseExec[cc.ID] = entry
	}
	schemaDigest, _ := sha256File(filepath.Join(root, "inputs.schema.json"))
	wfDigest, _ := sha256File(filepath.Join(root, "execution", "workflow.yml"))
	manifestDigest, _ := sha256File(filepath.Join(root, "agent.yaml"))

	closure := map[string]any{
		"schema":       "workagent/attest/closure/1",
		"generated_at": nowISO(),
		"principle":    i18n.T("摘要必须覆盖推理函数的完整计算闭包。任何落在被证明集合之外的组件，都是一条伪造通道。", "The digest must cover the full computational closure of the inference function. Any component outside the proven set is a forgery channel."),
		"closure": map[string]any{
			"executor_digest":      execDigest,
			"executor_command":     strings.Join(cmd, " "),
			"case_executors":       caseExec,
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
		"llm_used": llmUsed,
		"note":     i18n.T("确定性执行体不涉及模型闭包；一旦启用 LLM 步骤，model/tokenizer/chat_template/engine/decoding 字段必须填写，否则不能声称结论可复现。执行体自身的运行时版本（如 Python/Node）由执行体作者负责登记。", "A deterministic executor has no model closure. Once an LLM step is enabled, the model/tokenizer/chat_template/engine/decoding fields must be filled in, otherwise the conclusion cannot be called reproducible. The executor's own runtime version (Python/Node) is the executor author's responsibility to record."),
	}
	if err := writeJSON(filepath.Join(root, "attest", "closure.json"), closure); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	fmt.Println(i18n.T("attest —— 计算闭包摘要已写入 attest/closure.json", "attest — closure digests written to attest/closure.json"))
	ok(i18n.Tf("场景数 %d（执行体摘要逐场景记录）", "%d scenarios (executor digest recorded per scenario)", len(all)))
	ok("executor_digest      " + short(execDigest))
	ok("inputs_schema_digest " + short(schemaDigest))
	ok("workflow_digest      " + short(wfDigest))
	ok("manifest_digest      " + short(manifestDigest))
	ok("toolchain            " + toolchainVersion())
	info(fmt.Sprintf("llm_used = %v", closure["llm_used"]))
	return 0
}

// ---------------------------------------------------------------- run

func runExecutor(root string, c Case, inputPath string) (any, []string, error) {
	cmd := resolveExecCommand(c.Executor)
	if len(cmd) == 0 {
		return nil, nil, fmt.Errorf(i18n.T("场景 %s 未声明 executor", "scenario %s declares no executor"), c.ID)
	}
	args := append(append([]string{}, cmd[1:]...), inputPath)
	proc := exec.Command(cmd[0], args...)
	proc.Dir = root
	var stdout, stderr bytes.Buffer
	proc.Stdout = &stdout
	proc.Stderr = &stderr
	if err := proc.Run(); err != nil {
		return nil, cmd, fmt.Errorf(i18n.T("执行体失败 (%v): %s", "executor failed (%v): %s"), err, strings.TrimSpace(stderr.String()))
	}
	dec := json.NewDecoder(bytes.NewReader(stdout.Bytes()))
	dec.UseNumber()
	var payload any
	if err := dec.Decode(&payload); err != nil {
		return nil, cmd, fmt.Errorf(i18n.T("执行体输出不是合法 JSON: %v", "executor output is not valid JSON: %v"), err)
	}
	if pm, isMap := payload.(map[string]any); isMap {
		if out, exists := pm["output"]; exists {
			return out, cmd, nil
		}
	}
	return payload, cmd, nil
}

func cmdRun(root, caseID, inputArg string, replay bool, providerOverride, modelOverride string) int {
	m := mustManifest(root)
	c, found := findCase(m, caseID)
	if !found {
		fmt.Fprintln(os.Stderr, i18n.T("找不到场景: ", "Scenario not found: ")+caseID+i18n.T("（用 workagent cases 查看全部）", " (see all with: workagent cases)"))
		return 2
	}
	if inputArg == "" {
		// 没给 --input 就用场景自带的样例：结构化优先，其次自然语言。
		// 只带 sample_natural 的场景（有模型抽取的那些）以前会直接报错。
		switch {
		case c.SampleInput != "":
			inputArg = c.SampleInput
		case c.SampleNatural != "":
			inputArg = c.SampleNatural
		default:
			fmt.Fprintln(os.Stderr, i18n.T("run 需要 --input 参数（场景 ", "run needs --input (scenario ")+c.ID+i18n.T(" 未声明样例输入）", " declares no sample input)"))
			return 2
		}
	}
	inputPath := inputArg
	if !filepath.IsAbs(inputPath) {
		inputPath = filepath.Join(root, filepath.FromSlash(inputArg))
	}
	if !fileExists(inputPath) {
		fmt.Fprintln(os.Stderr, i18n.T("找不到输入文件: ", "Input file not found: ")+inputPath)
		return 2
	}

	inputObj, err := readJSON(inputPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, i18n.T("输入不是合法 JSON: ", "Input is not valid JSON: ")+err.Error())
		return 2
	}
	inCanon, _ := canonicalJSON(inputObj)
	inputDigest := sha256Bytes(inCanon)

	// LLM 步骤（可选）：在确定性执行体之前完成"抽取"，把非结构化输入变成结构化中间表示。
	// 执行体拿到的始终是确定性数据，因此裁决过程可复现。
	execInput := inputPath
	var llmDescriptors []map[string]any
	if len(c.LLMSteps) > 0 {
		// replay 模式绝不写档：否则重放会用降级的等级把原始录制覆盖掉。
		enriched, descs, lerr := runLLMSteps(root, c.ID, c.LLMSteps, inputObj, replay, providerOverride, modelOverride, !replay)
		if lerr != nil {
			fmt.Fprintln(os.Stderr, lerr.Error())
			return 1
		}
		llmDescriptors = descs
		if enriched != "" {
			execInput = enriched
		}
	}

	fmt.Println(i18n.T("场景: ", "Scenario: ") + c.ID + "  " + c.DisplayTitle())
	outputObj, cmd, err := runExecutor(root, c, execInput)
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
	riskText := i18n.T("- （无）", "- (none)")
	if len(riskLines) > 0 {
		riskText = strings.Join(riskLines, "\n")
	}
	execDigest := "n/a"
	if len(cmd) > 1 {
		if h, herr := sha256File(filepath.Join(root, filepath.FromSlash(cmd[1]))); herr == nil {
			execDigest = h[:16]
		}
	}
	// 通用发现渲染：[级别] 说明
	var findLines []string
	for _, f := range asList(outMap["findings"]) {
		fm := asMap(f)
		lv := asString(fm["level"])
		if lv == "" {
			lv = "info"
		}
		msg := asString(fm["message"])
		code := asString(fm["code"])
		if code != "" {
			msg = "[" + code + "] " + msg
		}
		findLines = append(findLines, "- **"+lv+"** "+msg)
	}
	findText := i18n.T("- （无）", "- (none)")
	if len(findLines) > 0 {
		findText = strings.Join(findLines, "\n")
	}
	subj := asString(outMap["subject"])
	if subj == "" {
		subj = asString(outMap["project"])
	}
	repl := map[string]string{
		"<case_id>":          c.ID,
		"<case_title>":       c.Title,
		"<subject>":          subj,
		"<verdict>":          asString(outMap["verdict"]),
		"<findings>":         findText,
		"<repro_level>":      overallLevel(llmDescriptors),
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
	trace["last_case"] = c.ID
	trace["steps"] = append(steps, map[string]any{
		"case":          c.ID,
		"seq":           len(steps) + 1,
		"at":            nowISO(),
		"step":          "run",
		"executor":      strings.Join(cmd, " "),
		"input_file":    relToRoot(root, inputPath),
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

	fmt.Println(i18n.T("run —— 执行完成", "run — completed"))
	ok(i18n.T("证据包编号 ", "Evidence bundle id ") + evidenceID)
	ok(i18n.T("结论: ", "Conclusion: ") + asString(outMap["conclusion"]))
	ok(i18n.T("输出摘要 ", "Output digest ") + outputDigest[:16])
	ok(i18n.T("复现等级 ", "Reproducibility level ") + overallLevel(llmDescriptors))
	if len(llmDescriptors) > 0 {
		info("证据包含模型响应留档；verify 默认重放记录而非重放模型")
	}
	info(i18n.T("证据目录 ", "Evidence directory ") + evDir)
	info(i18n.T("摘要文件 ", "Summary file ") + filepath.Join(root, "artifacts", "summary.md"))
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
		fmt.Println(i18n.T("verify —— 没有可复现的记录，请先运行 workagent run", "verify — no reproducible record yet; run `workagent run` first"))
		return 1
	}
	fmt.Println(i18n.T("verify —— 复现校验，证据包 ", "verify — reproducibility check, evidence bundle ") + asString(last["evidence_id"]))

	inputFile := resolveInputPath(root, asString(last["input_file"]))
	if !fileExists(inputFile) {
		inputFile = filepath.Join(root, evidenceDir, asString(last["evidence_id"]), "input.json")
	}
	if !fileExists(inputFile) {
		bad(i18n.T("找不到原始输入，无法复现", "original input not found; cannot reproduce"))
		return 1
	}

	// 从 trace 里取上次 run 用的场景（多场景包必须区分）
	lastCase := asString(asMap(traceRaw)["last_case"])
	vc, okc := findCase(m, lastCase)
	if !okc {
		fmt.Fprintln(os.Stderr, i18n.T("trace 中的场景无法解析: ", "scenario in trace could not be parsed: ")+lastCase)
		return 1
	}
	// 含 LLM 步骤时：默认重放已录制的模型响应（离线、可复现）；
	// --live 则重新真实调用模型，用来**测量**模型本身是否可复现。
	execInput := inputFile
	liveDiffers := false
	if len(vc.LLMSteps) > 0 {
		inObj, ierr := readJSON(inputFile)
		if ierr != nil {
			fmt.Fprintln(os.Stderr, ierr.Error())
			return 1
		}
		if live {
			// 关键：必须**先**读出旧录制的摘要，再重新调用。
			// 否则拿刚被覆盖的档去比对，永远相等、永远 PASS —— 那样的测试是摆设。
			oldDigests := map[string]string{}
			for _, st := range vc.LLMSteps {
				sid := asString(asMap(st)["id"])
				if rec, rerr := readJSON(filepath.Join(root, llmDir, vc.ID, sid+".json")); rerr == nil {
					oldDigests[sid] = asString(asMap(rec)["response_digest"])
				}
			}
			enriched, descs, lerr := runLLMSteps(root, vc.ID, vc.LLMSteps, inObj, false, "", "", false)
			if lerr != nil {
				fmt.Fprintln(os.Stderr, i18n.T("live 调用失败: ", "live call failed: ")+lerr.Error())
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
					fmt.Println("  [!!]   " + sid + i18n.T(" 没有历史录制，无法比较模型是否复现", " has no recorded response; cannot check whether the model reproduces"))
					continue
				}
				if old == neu {
					fmt.Println("  [OK]   " + sid + i18n.T(" 模型本次响应与录制**逐字节一致**（", " the model response is byte-identical to the recording (") + short(neu) + ")")
				} else {
					liveDiffers = true
					fmt.Println("  [!!]   " + sid + i18n.T(" 模型本次响应与录制不同", " the model response differs from the recording"))
					fmt.Println(i18n.T("         录制 ", "         recorded ") + short(old) + "  !=  " + i18n.T("本次 ", "this run ") + short(neu))
					fmt.Println(i18n.T("         —— 该模型在 temperature=0/seed 固定下**仍然不可复现**", "         — this model is NOT reproducible even with temperature=0 and a fixed seed"))
				}
			}
		} else {
			fmt.Println(i18n.T("  [--]   LLM 步骤：重放已录制响应（未联网）", "  [--]   LLM step: replaying the recorded response (offline)"))
			enriched, _, lerr := runLLMSteps(root, vc.ID, vc.LLMSteps, inObj, true, "", "", false)
			if lerr != nil {
				fmt.Fprintln(os.Stderr, lerr.Error())
				return 1
			}
			if enriched != "" {
				execInput = enriched
			}
		}
	}

	outputObj, _, err := runExecutor(root, vc, execInput)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}
	canon, _ := canonicalJSON(outputObj)
	replay := sha256Bytes(canon)
	expected := asString(last["output_digest"])
	if live && liveDiffers {
		fmt.Println(i18n.T("结果: FAIL（live）—— 模型本次未复现，因此该结论不可复现。请用 replay 模式交付。", "Result: FAIL (live) — the model did not reproduce, so the conclusion is not reproducible. Deliver it in replay mode."))
		return 1
	}

	fmt.Println(i18n.T("  期望摘要 ", "  Expected digest ") + expected)
	fmt.Println(i18n.T("  重放摘要 ", "  Replay digest   ") + replay)
	if replay == expected {
		fmt.Println(i18n.T("结果: PASS —— 同输入产出同结论，可复现。", "Result: PASS — same input, same conclusion. Reproducible."))
		return 0
	}
	fmt.Println(i18n.T("结果: FAIL —— 结论不可复现，禁止对外交付该结论。", "Result: FAIL — the conclusion is not reproducible. Do not deliver it."))
	return 1
}

// cmdVerifyAll 逐个场景复现校验：CI 里 8 个场景都要过。
func cmdVerifyAll(root string) int {
	m := mustManifest(root)
	traceRaw, err := readJSON(filepath.Join(root, "provenance", "trace.json"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	lastByCase := map[string]map[string]any{}
	var order []string
	for _, s := range asList(asMap(traceRaw)["steps"]) {
		sm := asMap(s)
		if asString(sm["step"]) != "run" {
			continue
		}
		cid := asString(sm["case"])
		if cid == "" {
			cid = "default"
		}
		if _, seen := lastByCase[cid]; !seen {
			order = append(order, cid)
		}
		lastByCase[cid] = sm
	}
	if len(order) == 0 {
		fmt.Println(i18n.T("verify --all —— trace 里没有 run 记录", "verify --all — no run records in the trace"))
		return 1
	}
	fmt.Print(i18n.Tf("verify --all —— 共 %d 个场景\n", "verify --all — %d scenarios\n", len(order)))
	failures := 0
	for _, cid := range order {
		st := lastByCase[cid]
		c, found := findCase(m, cid)
		if !found {
			bad(cid + i18n.T(" 场景无法解析", ": scenario could not be parsed"))
			failures++
			continue
		}
		inputFile := resolveInputPath(root, asString(st["input_file"]))
		if !fileExists(inputFile) {
			inputFile = filepath.Join(root, evidenceDir, asString(st["evidence_id"]), "input.json")
		}
		if !fileExists(inputFile) {
			bad(cid + i18n.T(" 找不到原始输入", ": original input not found"))
			failures++
			continue
		}
		execInput := inputFile
		if len(c.LLMSteps) > 0 {
			inObj, ierr := readJSON(inputFile)
			if ierr != nil {
				bad(cid + i18n.T(" 输入读取失败: ", ": failed to read input: ") + ierr.Error())
				failures++
				continue
			}
			enriched, _, lerr := runLLMSteps(root, c.ID, c.LLMSteps, inObj, true, "", "", false)
			if lerr != nil {
				bad(cid + i18n.T(" 重放失败: ", ": replay failed: ") + lerr.Error())
				failures++
				continue
			}
			if enriched != "" {
				execInput = enriched
			}
		}
		outputObj, _, rerr := runExecutor(root, c, execInput)
		if rerr != nil {
			bad(cid + i18n.T(" 执行失败: ", ": execution failed: ") + rerr.Error())
			failures++
			continue
		}
		canon, _ := canonicalJSON(outputObj)
		got := sha256Bytes(canon)
		want := asString(st["output_digest"])
		if got == want {
			ok(fmt.Sprintf("%-14s PASS  %s", cid, got[:16]))
		} else {
			bad(i18n.Tf("%-16s FAIL  expected %s got %s", "%-16s FAIL  expected %s got %s", cid, short(want), got[:16]))
			failures++
		}
	}
	fmt.Println("")
	if failures > 0 {
		fmt.Print(i18n.Tf("结果: %d 个场景未通过，禁止对外交付。\n", "Result: %d scenario(s) failed. Do not deliver this package.\n", failures))
		return 1
	}
	fmt.Print(i18n.Tf("结果: 全部 %d 个场景复现通过。\n", "Result: all %d scenarios reproduce.\n", len(order)))
	return 0
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
		"honesty_note":   i18n.T("digest-only 只证明『内容自生成后未变』，不证明『谁签的』。如需不可否认性，请用 GPG/Ed25519 对 package_digest 另行签名，并把公钥放入 signature/。", "digest-only proves only that the content is unchanged since it was generated. It does not prove who signed it. For non-repudiation, sign package_digest separately with GPG/Ed25519 and put the public key under signature/."),
	}
	if err := writeJSON(filepath.Join(root, "signature", "agent.sig"), sig); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	fmt.Println(i18n.T("seal —— 全包摘要已生成", "seal — whole-package digest generated"))
	ok("package_digest " + packageDigest)
	ok(i18n.Tf("文件数 %d", "%d files", len(entries)))
	info(i18n.T("scheme = digest-only（未冒充密码学签名）", "scheme = digest-only (no claim of a cryptographic signature)"))
	return 0
}

// ---------------------------------------------------------------- main

// versionOf 读指定交付包的版本，读不到才回落到编译期常量。
// （toolVersion 常量曾长期写死 0.3.0，而 manifest 早已是 0.9.x —— 打印错版本比不打印更糟。）
func versionOf(root string) string {
	if m := mustManifest(root); m != nil {
		if v := asString(m["version"]); v != "" {
			return v
		}
	}
	return toolVersion
}

// currentVersion 读当前目录交付包的版本。
func currentVersion() string { return versionOf(resolveRoot(".")) }

func usage() {
	fmt.Println(i18n.T("WorkAgent 包工具链 v", "WorkAgent package toolchain v") + currentVersion() + "\n\n" +
		i18n.T(
			"用法:\n"+
				"  workagent check  [path]                 校验包结构与兼容声明\n"+
				"  workagent adapt  [path]                 从 agent.yaml 生成适配层\n"+
				"  workagent attest [path]                 计算 Attestation Closure 摘要\n"+
				"  workagent run    --input FILE [path]    执行并产出证据包\n"+
				"  workagent run    --input FILE --replay  使用已录制的模型响应（离线）\n"+
				"  workagent models                        列出内置模型服务商与可用状态\n"+
				"  workagent verify [path]                 复现校验：同输入是否产出同结论\n"+
				"  workagent verify [path] --live          重新真实调用模型，测量其是否可复现\n"+
				"  workagent record --case ID [--input F]  跑一次并把结论+证据公开留档\n"+
				"  workagent mcp    [path]                 以 MCP server 方式暴露本包（stdio）\n"+
				"  workagent seal   [path]                 生成全包摘要",
			"Usage:\n"+
				"  workagent check  [path]                 validate package structure and compatibility\n"+
				"  workagent adapt  [path]                 generate the adapter layer from agent.yaml\n"+
				"  workagent attest [path]                 compute Attestation Closure digests\n"+
				"  workagent run    --input FILE [path]    execute and emit an evidence bundle\n"+
				"  workagent run    --input FILE --replay  use recorded model responses (offline)\n"+
				"  workagent models                        list built-in providers and availability\n"+
				"  workagent verify [path]                 reproducibility check: same input, same conclusion?\n"+
				"  workagent verify [path] --live          call the model again for real to measure reproducibility\n"+
				"  workagent record --case ID [--input F]  run once and archive the conclusion publicly\n"+
				"  workagent mcp    [path]                 expose this package as an MCP server (stdio)\n"+
				"  workagent seal   [path]                 produce the whole-package digest") + "\n\n" +
		i18n.T("path 默认为当前目录。", "path defaults to the current directory."))
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
	// 语言必须在**任何输出之前**定下来，而且要在**完整参数**上解析 ——
	// `workagent --lang en` 里 --lang 落在 os.Args[1]，只看 args（os.Args[2:]）会漏掉。
	raw := os.Args[1:]
	i18n.SetLang(i18n.Resolve(i18n.EnvLang(), raw))
	// 把 --lang 摘掉，否则它的取值会被当成位置参数或命令名。
	raw = i18n.StripLang(raw)
	if len(raw) == 0 {
		usage()
		os.Exit(0)
	}
	switch raw[0] {
	case "-v", "--version", "version":
		fmt.Println("workagent " + currentVersion())
		return
	}
	var code int
	args := raw[1:]
	switch raw[0] {
	case "check":
		code = cmdCheck(pathArg(args))
	case "adapt":
		code = cmdAdapt(pathArg(args))
	case "attest":
		code = cmdAttest(pathArg(args))
	case "cases":
		code = cmdCases(pathArg(args))
	case "models":
		probe := false
		for _, a := range args {
			if a == "--probe" || a == "-probe" {
				probe = true
			}
		}
		code = cmdModels(probe)
	case "run":
		// 手工解析：Go 的 flag 包遇到第一个位置参数就停止解析，
		// 于是 run --input X . --replay 会**静默忽略** --replay —— 那太危险了。
		// 这里做成位置无关：显式声明的开关一定生效。
		var inputArg, provider, model, caseID string
		replayFlag := false
		allFlag := false
		var positional []string
		for i := 0; i < len(args); i++ {
			a := args[i]
			switch {
			case a == "--replay" || a == "-replay":
				replayFlag = true
			case a == "--all" || a == "-all":
				allFlag = true
			case a == "--input" || a == "-input":
				if i+1 < len(args) {
					inputArg = args[i+1]
					i++
				}
			case strings.HasPrefix(a, "--input="):
				inputArg = strings.TrimPrefix(a, "--input=")
			case a == "--provider":
				if i+1 < len(args) {
					provider = args[i+1]
					i++
				}
			case strings.HasPrefix(a, "--provider="):
				provider = strings.TrimPrefix(a, "--provider=")
			case a == "--model":
				if i+1 < len(args) {
					model = args[i+1]
					i++
				}
			case strings.HasPrefix(a, "--model="):
				model = strings.TrimPrefix(a, "--model=")
			case a == "--case" || a == "-case":
				if i+1 < len(args) {
					caseID = args[i+1]
					i++
				}
			case strings.HasPrefix(a, "--case="):
				caseID = strings.TrimPrefix(a, "--case=")
			case !strings.HasPrefix(a, "-"):
				positional = append(positional, a)
			}
		}
		runRoot := resolveRoot(".")
		if len(positional) > 0 {
			runRoot = resolveRoot(positional[0])
		}
		if allFlag {
			code = cmdRunAll(runRoot, replayFlag)
			break
		}
		code = cmdRun(runRoot, caseID, inputArg, replayFlag, provider, model)
	case "verify":
		live := false
		all := false
		var vpos []string
		for _, a := range args {
			switch {
			case a == "--live" || a == "-live":
				live = true
			case a == "--all" || a == "-all":
				all = true
			case !strings.HasPrefix(a, "-"):
				vpos = append(vpos, a)
			}
		}
		if all {
			code = cmdVerifyAll(resolveRoot("."))
			break
		}
		verifyRoot := resolveRoot(".")
		if len(vpos) > 0 {
			verifyRoot = resolveRoot(vpos[0])
		}
		code = cmdVerify(verifyRoot, live)
	case "record":
		var caseID, inputArg, note string
		recLive := false
		var recPos []string
		for i := 0; i < len(args); i++ {
			a := args[i]
			switch {
			case a == "--case" || a == "-case":
				if i+1 < len(args) {
					caseID = args[i+1]
					i++
				}
			case strings.HasPrefix(a, "--case="):
				caseID = strings.TrimPrefix(a, "--case=")
			case a == "--input" || a == "-input":
				if i+1 < len(args) {
					inputArg = args[i+1]
					i++
				}
			case strings.HasPrefix(a, "--input="):
				inputArg = strings.TrimPrefix(a, "--input=")
			case a == "--live" || a == "-live":
				recLive = true
			case a == "--note" || a == "-note":
				if i+1 < len(args) {
					note = args[i+1]
					i++
				}
			case strings.HasPrefix(a, "--note="):
				note = strings.TrimPrefix(a, "--note=")
			case !strings.HasPrefix(a, "-"):
				recPos = append(recPos, a)
			}
		}
		recRoot := resolveRoot(".")
		if len(recPos) > 0 {
			recRoot = resolveRoot(recPos[0])
		}
		code = cmdRecord(recRoot, caseID, inputArg, note, recLive)
	case "mcp":
		code = cmdMCP(pathArg(args))
	case "seal":
		code = cmdSeal(pathArg(args))
	default:
		usage()
		code = 1
	}
	os.Exit(code)
}
