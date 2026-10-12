package main

// workagent mcp —— 把交付包本身暴露成一个 MCP server（stdio，换行分隔 JSON-RPC）。
//
// 这样 WorkAgent 不只属于 DSH：任何 MCP 客户端（Claude Desktop / Cursor / …）
// 都能把「跑场景 → 拿可复现结论」当成一个工具来调用。
//
// 工具实现直接复用 CLI 自身：用 os.Executable() 起子进程并捕获输出，
// 保证「MCP 调用」与「命令行调用」走的是同一份逻辑，不会出现两套行为。

import (
	"bufio"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/WmjXiaoJun/workagent/internal/i18n"
)

type mcpRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

func mcpWrite(v any) {
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	os.Stdout.Write(b)
	os.Stdout.Write([]byte("\n"))
}

func mcpResult(id json.RawMessage, result any) {
	mcpWrite(map[string]any{"jsonrpc": "2.0", "id": json.RawMessage(id), "result": result})
}

func mcpError(id json.RawMessage, code int, msg string) {
	mcpWrite(map[string]any{"jsonrpc": "2.0", "id": json.RawMessage(id),
		"error": map[string]any{"code": code, "message": msg}})
}

func mcpText(id json.RawMessage, text string, isErr bool) {
	mcpResult(id, map[string]any{
		"content": []any{map[string]any{"type": "text", "text": text}},
		"isError": isErr,
	})
}

// runCLI 起一个自己的子进程执行 CLI 子命令，捕获输出。
// 用子进程而不是直接调用，是因为 CLI 函数都直接写 stdout —— 那会污染 MCP 的 JSON-RPC 流。
func runCLI(root string, args ...string) (string, bool) {
	exe, err := os.Executable()
	if err != nil {
		return i18n.T("无法定位自身可执行文件: ", "cannot locate own executable: ") + err.Error(), true
	}
	full := append([]string{}, args...)
	if root != "" {
		full = append(full, root)
	}
	proc := exec.Command(exe, full...)
	if root != "" {
		proc.Dir = root
	}
	out, err := proc.CombinedOutput()
	return string(out), err != nil
}

func mcpTools() []any {
	obj := func(props map[string]any, required ...string) map[string]any {
		m := map[string]any{"type": "object", "properties": props}
		if len(required) > 0 {
			m["required"] = required
		}
		return m
	}
	rootProp := map[string]any{"type": "string",
		"description": i18n.T("交付包根目录（含 agent.yaml）。省略时用当前工作目录。",
			"Delivery package root directory (containing agent.yaml). Defaults to the current working directory.")}
	return []any{
		map[string]any{
			"name": "workagent_cases",
			"description": i18n.T("列出 WorkAgent 交付包里的可验证场景，按交付形态分组（测算型 / 核验型 / …），并给出每个场景的复现等级。",
				"List the verifiable scenarios in a WorkAgent delivery package, grouped by delivery form (Estimation / Verification / …), with each scenario's reproducibility level."),
			"inputSchema": obj(map[string]any{"root": rootProp}),
		},
		map[string]any{
			"name": "workagent_run",
			"description": i18n.T("运行一个场景并返回结论与证据包编号。结论是可复现的：同一输入任何时候跑都得到同样的输出摘要。",
				"Run one scenario and return its conclusion and evidence-bundle id. The conclusion is reproducible: the same input yields the same output digest at any time."),
			"inputSchema": obj(map[string]any{
				"root":    rootProp,
				"case_id": map[string]any{"type": "string", "description": i18n.T("场景 ID，例如 spc_control、modbus_poll。", "Scenario id, e.g. spc_control, modbus_poll.")},
				"input":   map[string]any{"type": "string", "description": i18n.T("输入 JSON 路径。省略时用该场景自带的样例输入。", "Path to the input JSON. Defaults to the scenario's bundled sample input.")},
				"replay":  map[string]any{"type": "boolean", "description": i18n.T("带模型的场景用已录制响应重放，不需要联网与密钥。", "For scenarios with model steps, replay a recorded response — no network or API key needed.")},
			}, "case_id"),
		},
		map[string]any{
			"name": "workagent_verify",
			"description": i18n.T("复现校验：重放每个场景并比对结论摘要，全部 PASS 才说明交付包可对外交付。",
				"Reproducibility check: replay every scenario and compare conclusion digests. All PASS is what makes the package deliverable."),
			"inputSchema": obj(map[string]any{
				"root": rootProp,
				"live": map[string]any{"type": "boolean", "description": i18n.T("为 true 时重新真实调用模型，测量它到底复现不复现。", "When true, calls the model again for real to measure whether it actually reproduces.")},
			}),
		},
	}
}

func mcpCall(root string, params json.RawMessage) (string, bool) {
	var p struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return i18n.T("无法解析 tools/call 参数: ", "cannot parse tools/call params: ") + err.Error(), true
	}
	var args struct {
		Root   string `json:"root"`
		CaseID string `json:"case_id"`
		Input  string `json:"input"`
		Replay bool   `json:"replay"`
		Live   bool   `json:"live"`
	}
	if len(p.Arguments) > 0 {
		_ = json.Unmarshal(p.Arguments, &args)
	}
	target := root
	if args.Root != "" {
		target = args.Root
	}
	// 会话的工作目录未必是交付包。与其让子进程抛一句裸错误，
	// 不如直接告诉调用方该改什么。
	if !fileExists(filepath.Join(target, "agent.yaml")) {
		return i18n.T("当前目录不是 WorkAgent 交付包（缺 agent.yaml）：",
			"The working directory is not a WorkAgent package (no agent.yaml): ") + target +
			i18n.T("。把插件配置里的 args 指向一个交付包目录即可。",
				". Point the plugin's args at a package directory."), true
	}
	switch p.Name {
	case "workagent_cases":
		out, isErr := runCLI(target, "cases")
		return out, isErr
	case "workagent_run":
		if strings.TrimSpace(args.CaseID) == "" {
			return i18n.T("缺少 case_id 参数", "missing case_id argument"), true
		}
		cmdArgs := []string{"run", "--case", args.CaseID}
		if args.Input != "" {
			cmdArgs = append(cmdArgs, "--input", args.Input)
		}
		if args.Replay {
			cmdArgs = append(cmdArgs, "--replay")
		}
		out, isErr := runCLI(target, cmdArgs...)
		return out, isErr
	case "workagent_verify":
		cmdArgs := []string{"verify", "--all"}
		if args.Live {
			cmdArgs = append(cmdArgs, "--live")
		}
		out, isErr := runCLI(target, cmdArgs...)
		return out, isErr
	default:
		return i18n.T("未知工具: ", "unknown tool: ") + p.Name, true
	}
}

// manifestVersion 从 agent.yaml 实时读版本，避免常量与真源漂移
// （toolVersion 常量已经漂过一次：写死 0.3.0，而 manifest 早就是 0.7.x）。
// manifestVersion 尽力读版本，**读不到不算错**。
//
// MCP 服务端必须在"当前目录不是交付包"时也能正常启动 —— 否则一个 cwd 不巧的会话
// 会让 MCP 客户端每次启动都失败（插件配的是 failOnStartupError: true）。
// 包的问题应该在**具体工具调用**上暴露，而不是在进程启动时。
func manifestVersion(root string) string {
	b, err := os.ReadFile(filepath.Join(root, "agent.yaml"))
	if err != nil {
		return ""
	}
	if m, isMap := parseYAML(string(b)).(map[string]any); isMap {
		if v := asString(m["version"]); v != "" {
			return v
		}
	}
	return ""
}

// cmdMCP 在 stdin/stdout 上提供 MCP（stdio 传输：一行一个 JSON-RPC 消息）。
func cmdMCP(root string) int {
	ver := manifestVersion(root)
	if ver == "" {
		ver = toolVersion
	}
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 1<<20), 16<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var req mcpRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			mcpError(json.RawMessage("null"), -32700, i18n.T("JSON 解析失败: ", "JSON parse error: ")+err.Error())
			continue
		}
		switch req.Method {
		case "initialize":
			// 回显客户端请求的协议版本，兼容新老客户端。
			proto := "2025-06-18"
			var ip struct {
				ProtocolVersion string `json:"protocolVersion"`
			}
			if len(req.Params) > 0 {
				if err := json.Unmarshal(req.Params, &ip); err == nil && ip.ProtocolVersion != "" {
					proto = ip.ProtocolVersion
				}
			}
			mcpResult(req.ID, map[string]any{
				"protocolVersion": proto,
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo":      map[string]any{"name": "workagent", "version": ver},
			})
		case "notifications/initialized", "notifications/cancelled":
			// 通知无需响应
		case "ping":
			mcpResult(req.ID, map[string]any{})
		case "tools/list":
			mcpResult(req.ID, map[string]any{"tools": mcpTools()})
		case "tools/call":
			text, isErr := mcpCall(root, req.Params)
			mcpText(req.ID, text, isErr)
		default:
			mcpError(req.ID, -32601, i18n.T("不支持的方法: ", "unsupported method: ")+req.Method)
		}
	}
	return 0
}
