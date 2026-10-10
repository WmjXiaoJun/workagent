# WorkAgent — DeepSeek Harness 插件

把一个 WorkAgent 交付包作为 **MCP 工具**暴露给 Harness。装好之后，
你在对话里就能直接说"跑一下积分对账"，agent 会调用它并拿到**可复现的结论**。

## 它做什么

插件本身**不含逻辑**——它只是一段 5 行的配置，把 `workagent mcp`
这个 stdio MCP server 接到 Harness 上。真正的能力在交付包里。

| 工具 | 作用 |
|---|---|
| `workagent_cases` | 列出交付包里的场景，按**交付形态**分组 |
| `workagent_run` | 跑一个场景，返回结论 + 证据包编号 |
| `workagent_verify` | 复现校验：全部场景 PASS 才说明可交付 |

## 为什么走 MCP 而不是写 Cordis 插件

因为 **MCP 的服务端可以离线自测**，而 Cordis 宿主插件必须在真实 Harness 里才能验证。
而且走 MCP 还能顺带支持 Claude Desktop、Cursor 等任何 MCP 客户端——
一次实现，多处可用。

## 前置条件

```bash
# 1) 有 workagent 可执行文件（单文件静态二进制，无依赖）
workagent --help

# 2) 自测 MCP server 是否正常
printf '%s\n' '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | workagent mcp .
```

## 安装

在 Harness 里用 `plugin_manager` 的 `install_bundle`，
target 指向**本目录的绝对路径**：

```text
plugin_manager  action: install_bundle
                target: <本目录绝对路径>
```

## 配置

改 `cordis.patch.yml`：

| 字段 | 说明 |
|---|---|
| `command` | `workagent` 可执行文件；PATH 里没有就写绝对路径 |
| `args` | `["mcp", "."]` —— 第二个参数是交付包根目录，改成你的包路径 |
| `cwd` | 可选，指定工作目录 |

## ⚠️ 验证状态

- ✅ **MCP server 侧已验证**：握手、`tools/list`、`tools/call`、未知工具报错，全部实测通过
- ⚠️ **Cordis 装载侧未验证**：我没有 `plugin_manager` 工具，无法在本机安装，
  所以**"装上去以后 Harness 里能用"这件事我无法声称**。装完请自行确认一次。

## 卸载

`plugin_manager` 的 `remove_bundle`。
