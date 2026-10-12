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

### 方式一：从 npm 装（推荐）

DSH 的 `plugin_manager.installBundle` **本来就支持 npm registry**
（还带 npmmirror 兜底），所以发包到 npm 就是 DSH 生态的正规分发路径：

```text
plugin_manager  action: install_bundle
                target: dsh-plugin-workagent
```

### 方式二：从本地目录装

target 指向**本目录的绝对路径**（适合开发与改配置）：

```text
plugin_manager  action: install_bundle
                target: <本目录绝对路径>
```

### 方式三：从 GitHub 装

`installBundle` 会先用 `git ls-remote` 检查仓库再交给 pnpm：

```text
plugin_manager  action: install_bundle
                target: github:WmjXiaoJun/workagent#path:dsh-plugin
```

> 具体 spec 写法以你所用版本的 `plugin_manager` 为准；前两种是实测存在的路径。

## 配置

改 `cordis.patch.yml`：

| 字段 | 说明 |
|---|---|
| `command` | `workagent` 可执行文件；PATH 里没有就写绝对路径 |
| `args` | `["mcp", "."]` —— 第二个参数是交付包根目录，改成你的包路径 |
| `cwd` | 可选，指定工作目录 |

## 发布到 npm

包已配置为可发布（`private` 已移除、补了 license/author/repository/keywords/files`）。

```bash
cd dsh-plugin
npm login                       # 需要你自己的 npm 账号
npm publish --access public     # 无作用域包可省略 --access
```

发布前自检（都不需要登录）：

```bash
npm pack --dry-run              # 看实际会打进去哪些文件
npm publish --dry-run           # 走完发布流程但不真发
```

### ⚠️ 一个必须知道的前提

**这个包只含配置，不含 `workagent` 可执行文件。**
DSH 安装依赖时会**默认拦截包的安装脚本**（pnpm 11 的 `pendingBuilds` 机制），
所以它**无法在安装时自动下载二进制**。

装完插件后 `workagent` 必须在 `PATH` 上，否则 MCP server 起不来
（`failOnStartupError: true` 会**明确报错**，而不是静默失效——这是故意的）。

```bash
# 从 GitHub Releases 取对应平台的静态二进制
# https://github.com/WmjXiaoJun/workagent/releases/latest
workagent --help
```

## 验证状态

| 层 | 状态 |
|---|---|
| **MCP server** | ✅ 握手、`tools/list`、`tools/call`、未知工具报错，全部直接实测 |
| **Cordis 组合** | ✅ 装进一次性 profile，组合出来的配置里出现了下面这一段 |
| **运行时连接** | ⚠️ 未实测 —— 需要真实会话里由 DSH 的 MCP 客户端真正拉起进程 |

```yaml
# == dsh-plugin-workagent
- id: workagent-mcp
  name: '@deepseek-ai/dsh-mcp-client'
  config:
    serverName: workagent
    transport: stdio
    command: workagent
```

`dsh --profile <名字> --dump-config` 能在**不启动任何东西**的情况下打出这棵树，组合就是这样核对的。
一次性 profile 事后已删除。

### ⚠️ 顺序很重要：先把 `workagent` 放进 `PATH`

插件设了 `failOnStartupError: true`。如果 `workagent` 还**不在** `PATH` 上就装进某个 profile，
MCP 客户端会**每次启动都失败** —— 那等于把"缺一个前提"变成了"反复报错"。

所以：先装好二进制、确认 `workagent --help` 能跑，**再**装插件。

## 卸载

`plugin_manager` 的 `remove_bundle`。
