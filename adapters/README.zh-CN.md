# adapters/

**生成物，不是真源。** 运行 `workagent adapt` 生成：

| 文件 | 目标生态 | 说明 |
|---|---|---|
| `plugin.json` | Agent Plugins 1.0（Vercel 牵头，OpenAI/AWS/GitHub/Microsoft 支持） | 可移植插件清单 |
| `agent-card.json` | A2A（Agent2Agent） | Agent 能力与身份自描述，供发现 |

真源永远是根目录的 `agent.yaml`。改适配层等于白改，下次生成会被覆盖。
