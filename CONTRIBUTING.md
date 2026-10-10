# 贡献指南

## 我们最需要什么

| 优先级 | 需要 | 说明 |
|---|---|---|
| P0 | **真实场景的执行体** | 一个具体领域、能验真的最小场景，比十个通用 demo 有价值 |
| P0 | **反例**：结论不可复现的案例 | 帮我们把"可复现"的门槛说清楚 |
| P1 | 兼容性报告 | 你的包在某个老平台上跑起来了吗？哪些能力丢了？ |
| P1 | 标准对齐的适配器 | Agent Plugins / A2A / 其它 |
| P2 | 文档翻译 | README.en.md 的改进 |

## 我们**不收**什么

- 通用 AI 助手 / 聊天机器人
- 平台、框架、编排引擎
- 引入第三方运行时依赖的核心工具（工具链必须保持零依赖）
- 无法验真的场景（结论对错说不清的领域）

## 提交前自检

```bash
python tools/workagent.py check .
python tools/workagent.py run --input examples/bid_quote/input.json .
python tools/workagent.py verify .      # 必须 PASS
```

**`verify` 不通过 = 不可复现 = 不允许合并。**

## 设计约束（改动前请先读）

1. **真源唯一**：`agent.yaml`。`adapters/` 永远是生成物，不接受手改。
2. **零依赖**：`tools/workagent.py` 只用 Python 标准库。
3. **只加不改**：兼容性靠"新增目录"，不靠"修改已有字段"。
4. **诚实**：没有私钥就不许写"已签名"；不确定就写"不确定"。

## 提交信息

```
<type>: <一句话说明>
```

type 取 `spec` / `tool` / `example` / `docs` / `compat` / `fix`。

## 许可

贡献即表示你同意以 MIT 许可发布你的贡献。
