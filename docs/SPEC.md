# WorkAgent 包规范 v0.3.0

> 状态：早期草案，接口可能变动 ｜ 本文描述 **v0.3.0** 的包格式与契约。

## 1. 术语

| 术语 | 含义 |
|---|---|
| **包（Package）** | 一个目录，含 `agent.yaml` 与必需文件 |
| **真源（Source of Truth）** | `agent.yaml`。唯一可手改的元数据入口 |
| **适配层（Adapters）** | 由真源生成、面向其它生态的清单文件 |
| **执行体（Executor）** | 真正产出结论的程序，遵守第 10 节契约 |
| **证据包（Evidence Bundle）** | 一次执行产出的输入、结论、摘要与元数据 |
| **结论摘要（Output Digest）** | 结论规范化 JSON 的 SHA-256 |
| **计算闭包（Attestation Closure）** | 复现结论所需的全部组件摘要集合 |

## 2. 包模型与必需文件

文件清单以 `cmd/workagent/main.go` 的 `requiredFiles` 为准（单一事实来源）。`check` 会逐项校验。

关键约定：

1. 包根目录必须有 `agent.yaml`。
2. 所有 JSON 文件必须是合法 UTF-8 JSON。
3. `adapters/` 与 `out/` 是生成物，**不入库**。
4. 除 `adapters/` 外，不得存在由工具生成又被人工编辑的文件。

## 3. 真源与适配层

- 真源：`agent.yaml`。
- `adapt` 读取真源，生成：
  - `adapters/plugin.json` —— 面向 Agent Plugins 1.0 客户端
  - `adapters/agent-card.json` —— 面向 A2A 发现
- **不变量**：任何命令都不得修改 `agent.yaml` 的内容（`attest` 只读取其哈希）。

## 4. 兼容与降级（`compat.json`）

`compat.json` 必须包含：

| 字段 | 必需 | 说明 |
|---|---|---|
| `principle` | 是 | 兼容策略的一句话声明 |
| `min_platform` | 是 | 最低平台要求 |
| `capability_matrix` | 是 | 每项能力在"旧平台"上的表现：`full` / `degraded` / `lost` |
| `degradation` | 是 | 降级后的保留项、丢失项与**明确声明** |

**规范性要求**：`capability_matrix` 中任何标为 `lost` 的能力，**不得**在 `degradation.statement` 中被描述为仍然可用。

## 5. 溯源

### `provenance/trace.json`
每次 `run` 追加一条记录，必须含：`input_digest`、`output_digest`、`evidence_id`、`executor`、`at`。`verify` 据此重放。

### `provenance/data_sources.json`
每个进入结论的数据源必须登记来源、快照版本、获取时间、哈希、访问权限。**未登记的数据源不得参与计算。**

### `provenance/assumptions.json`
每条假设必须含：`statement`（陈述）、`basis`（依据）、`impact`（影响等级）、`if_false`（失效后果）。另需 `excluded` 列出**排除场景**。

**规范性要求**：任何未登记的隐含假设，视为包的缺陷。

## 6. Attestation Closure

`attest/closure.json` 的 `closure` 必须覆盖：

- 执行体、输入 schema、工作流、清单的摘要
- 运行环境：构建工具链版本、平台、数值环境（`PYTHONHASHSEED` / `LC_ALL` / `TZ`）
- 执行体命令（`executor_command`）；**执行体自身的运行时版本**（如 Python / Node）由执行体作者负责登记

**若启用 LLM 步骤**，还必须填写：
`model_weights_digest`、`tokenizer_digest`、`chat_template_digest`、`engine_build_digest`、`decoding_config`。

**规范性要求**：只要上述任一 LLM 字段为空，`llm_used` 为真时，该包**不得**声称结论可复现。

## 7. 边界

`x-workagent/boundary.json` 必须含 `valid_when`、`invalid_when`、`confidence_rating`、`stop_condition`。

**规范性要求**：命中 `invalid_when` 时，执行体必须**以非零退出码终止**，且不得向 stdout 输出结论。

## 8. 测试套件

`x-workagent/test_suite/` 下四类：

| 文件 | 目的 |
|---|---|
| `unit.json` | 单节点逻辑校验 |
| `regression.json` | 固定历史基准，校验迭代偏差 |
| `sensitivity.json` | 参数波动下结论稳定性 |
| `stress.json` | 极端与边界输入 |

> ⚠️ **现状**：v0.3.0 中这四类仅为**声明**，`workagent.py` 尚未执行它们。列为 Roadmap。

## 9. 签名

`signature/agent.sig` 由 `seal` 生成，含 `scheme`、`package_digest` 与文件级哈希清单。

**规范性要求**：`scheme` 必须如实标注。未使用私钥时必须是 `digest-only`，**不得**使用 `signed` 等字样。

## 10. 执行体契约

```text
输入: argv[1] = 输入 JSON 路径
成功: 退出码 0，stdout 输出 {"output": {...}}（合法 JSON）
失败: 退出码非 0，stderr 输出原因
```

约束：

- 必须**确定性**（无随机、无网络、无时间依赖），或在 closure 中登记不可复现的部分。
- 不得向 stdout 输出除结果 JSON 之外的内容。

## 11. 版本与变更策略

- 遵循语义化版本。
- **只加不改**：兼容性靠新增目录/字段，不靠修改既有字段语义。
- 破坏性变更必须提升主版本号，并在 `CHANGELOG.md` 说明迁移路径。

## 12. LLM 步骤与复现等级

### 12.1 声明

`agent.yaml` 的 `execution.llm_steps` 声明模型步骤：

```yaml
llm_steps:
  - id: "extract"
    provider: "ollama"              # 见 workagent models
    model: "qwen2.5:7b"
    prompt: "path/to/prompt.md"     # 支持 {{INPUT_JSON}} 占位符
    temperature: 0
    seed: 42
    json_mode: true
    when_input_has: "description"   # 输入中无该字段则跳过（结构化输入零联网）
```

### 12.2 契约

1. 模型步骤在确定性执行体**之前**执行；
2. 模型输出被解析为结构化中间表示，与原始输入一起写入 `out/run/input.enriched.json`；
3. 执行体读到的**必须是结构化数据** —— 这是裁决可复现的前提；
4. 模型原始响应写入 `provenance/llm/<id>.json`，含 `prompt_digest` 与 `response_digest`。

### 12.3 重放

`run --replay` / `verify` 使用录制响应，**不联网**。

**规范性要求**：重放前必须校验 `prompt_digest`；不一致时必须**报错退出**，不得静默使用过期录制。

### 12.4 复现等级

| 等级 | 含义 | 条件 |
|---|---|---|
| L3 | 完全可复现 | 无 LLM 步骤 |
| L2 | 决策可复现 | 本地开权重服务商 + `temperature=0` + 固定 `seed` |
| L1 | 记录可复现 | 其余情况；裁决过程可重放，模型本身不可复现 |
| L0 | 不可复现 | `temperature>0` |

**规范性要求**：等级必须如实写入 `evidence.json` 的 `reproducibility` 字段。
`verify --live` 若测得模型本次响应与录制不同，必须报告 FAIL，**不得**降级为警告。

### 12.5 留档不可被校验破坏

**规范性要求**：`verify --live` 与其他校验路径**不得覆盖** `provenance/llm/` 下已有的录制。
