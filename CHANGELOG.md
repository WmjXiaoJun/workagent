# Changelog

本项目遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

## [0.6.2] - 2026-10-10

**改为按交付形态组织，去掉"行业"维度。**

### Changed
- **移除 `industry` 字段**：场景不再按行业标注。行业是结果不是依据；按行业分会让读者以为"这是八个行业的模板"，
  而真正的组织维度是**交付形态**（测算型 / 核验型 / 抽取型 / 排程型 / 推演型）。理由见 `docs/VERTICALS.md`。
- `workagent cases` 改为**按形态分组输出**，形态顺序固定：测算 → 核验 → 抽取 → 排程 → 推演
- 各场景 README 去掉 `**行业**` 元信息与标题里的行业前缀
- 结论摘要模板去掉"行业"行；`docs/EXAMPLES.md` 重写为按形态组织，并标注**尚未覆盖的三种形态及原因**

### 现状
| 形态 | 场景数 |
|---|---|
| 测算型 | 3 |
| 核验型 | 5 |
| 抽取型 / 排程型 / 推演型 | 待补 |

> 推演型**不推荐优先做**：它的结论无法事后验真，与本项目的立身之本冲突。

## [0.6.0] - 2026-10-10

**多场景框架 + 八个可验证场景 + 模型适配层被测试覆盖。**

### Added
- **多场景框架**：`agent.yaml` 的 `execution.cases`，一个包承载多个可验证场景，共用兼容声明 / 溯源 / 证据包 / 复现校验
- `workagent cases`：**按交付形态分组**列出全部场景（形态 / ID / 复现等级 / 标题）
- `workagent run --case <ID>`：按场景运行
- `workagent verify --all`：**逐个场景复现校验**，CI 里 8 个场景必须全 PASS
- **八个行业场景**：
  - `salary_tax` 人力资源 · 个税累计预扣（L3）
  - `loan_dscr` 金融 · 偿债能力测算（L3）
  - `spc_control` 制造业 · SPC 判异 8 准则（L3）
  - `pin_conflict` 嵌入式 · 引脚/中断/DMA 冲突（L3）
  - `bid_quote` 建筑工程 · 报价测算（L3/L2）
  - `expense_audit` 财务 · 报销合规（L2/L1）
  - `lab_report` 医疗 · 检验异常判定（L2/L1）
  - `point_table` 工业自动化 · Modbus 点表核对（L2/L1）
- **模型适配层单元测试**：三种协议族（OpenAI 兼容 / Anthropic / Gemini）用 `httptest` 覆盖，**无需 API key、不联网**即可在 CI 验证
- `docs/EXAMPLES.md` 场景清单与规则来源；`docs/PROVIDERS.md` 22 家服务商接入指南
- 通用结论摘要模板（`subject / verdict / findings / metrics / confidence` 统一契约）

### Fixed
- **CI 在 macOS 上构建测试二进制时缺少 `LC_UUID`，dyld 直接 abort trap**：统一改用 `CGO_ENABLED=0` 静态构建（与 `go build` 保持一致）。本项目不需要 cgo，静态构建同时避免了另一类偶发崩溃。
- **LLM 录制按场景分目录**：四个场景的步骤都叫 `extract`，原先共用 `provenance/llm/extract.json` 会互相覆盖
- `cmdRun` 中 `ok` 变量遮蔽同名辅助函数
- J 编辑引入的两处 Go 字符串换行转义错误

### Design notes
- 阈值一律放在**输入或规则文件**里，不写死在代码中（`loan_dscr` 的 1.25/1.10 可覆盖）
- 医疗场景的参考区间明确标注为**示例值**，并在结论里声明不构成医学诊断
- 点表场景补上 **Modbus 参考地址归一化**（`40001` → 偏移 0），不归一化会造成大面积误判

## [0.5.0] - 2026-10-10

**模型适配层：22 家服务商 + 可复现的 LLM 步骤。**

### Added
- `internal/llm/` 模型适配层（零第三方依赖）：22 个内置 profile，覆盖 OpenAI / Anthropic / Gemini / Mistral / xAI / Groq / Together / OpenRouter，国内 DeepSeek / 通义 / Kimi / 智谱 / 豆包 / 混元 / 百川 / MiniMax / 星火 / 硅基流动，以及本地 Ollama / vLLM / LM Studio
- `workagent models`：列出全部服务商、端点、所需环境变量与可用状态
- `agent.yaml` 的 `execution.llm_steps`：声明式 LLM 步骤，支持 `when_input_has` 条件执行
- **模型响应留档 + 重放**：`provenance/llm/<step>.json` 记录原始响应；`run --replay` 离线复现
- `verify --live`：重新真实调用模型，**测量**模型本身是否可复现
- **复现等级 L0–L3** 如实写入 `evidence.json`
- `docs/VERTICALS.md`：行业划分框架（五种交付物形态 + 四道筛选）
- 示例新增自然语言路径 `examples/bid_quote/input.natural.json`

### 实测结论
本地 Ollama + `qwen2.5:7b`、`temperature=0`、`seed=42` 下，**三次独立调用返回逐字节相同的响应**
（`response_digest 4d6abc79490d3241…`），该路径达到 **L2 决策可复现**。这是实测值，不是声称值。

### Fixed
- `verify --live` 曾先覆盖录制再与刚写入的记录比对，导致**永远 PASS** 的假测试；现改为先读旧录制再调用
- `llm.dig()` 不支持数组下标，导致 OpenAI 兼容响应的 content 取值为空
- `run --replay` 时不应改写 `provenance/` 下的录制文件

### Design notes
- **模型只做抽取，判断交给代码**：执行体拿到的永远是结构化数据，所以裁决过程可复现
- `verify` 默认重放录制而非重放模型；`--live` 才真实调用，且**不覆盖**已有录制

## [0.4.0] - 2026-10-10

**工具链从 Python 重写为 Go。**

### Changed
- `tools/workagent.py` → `cmd/workagent/`（`main.go` + `yaml.go`）
- 分发方式：由"需要 Python 运行时"变为**单个静态二进制**，支持 `GOOS`/`GOARCH` 交叉编译
- `attest/closure.json` 的 `python_version` 字段改为 `toolchain`（如实记录构建工具链）+ 新增 `executor_command`
- Dockerfile 改为两阶段构建：Go 静态二进制 + 执行体运行时

### Added
- `Makefile`：`make release` 一条命令产出五平台二进制
- CI 扩展到三操作系统矩阵

### Why Go
本工具是 I/O 密集型（读文件、算 SHA-256、起子进程），Rust 的性能优势用不上，
却要引入 `serde@@ / `sha2@@ 等 crate，违背"零第三方依赖"这条铁律。
Go 的标准库覆盖全部需求，同时解决最大的痛点：**收到包的人不需要装任何运行时就能验证。**

### Compatibility
重写后**摘要算法与 Python 版完全一致** —— 用同一个输入验证，证据包编号与结论摘要逐字节相同
（`6723c555d11c0a3e` / `8e38afbabb710b00...`）。既有证据包无需重建。

> 已删除 `tools/workagent.py`。**刻意不同时保留两个实现**——两份实现一旦行为分叉，
> 对一个"可复现"项目是致命的。历史版本见 commit `0c77582`。

## [0.3.0] - 2026-10-10

个人可验证版首发。

### Added
- `agent.yaml` 单一真源清单（APS 基础字段 + `x-workagent` 扩展区）
- `compat.json` **兼容降级声明**：显式说明旧平台会丢失哪些能力（本项目的差异化能力）
- `attest/closure.json` **Attestation Closure**：计算闭包摘要，覆盖执行体 / schema / 工作流 / 运行环境
- 零依赖工具链：`check` / `adapt` / `attest` / `run` / `verify` / `seal`
- `adapters/` 适配层生成：Agent Plugins 1.0 `plugin.json` + A2A `agent-card.json`
- `provenance/` 溯源三件套：`trace.json` / `data_sources.json` / `assumptions.json`
- `x-workagent/` 扩展层：`boundary.json` + 四类测试套件
- 示例场景：投标报价测算（`examples/bid_quote/`）
- `docs/SPEC.md` 规范说明、`docs/COMPAT.md` 兼容指南

### Design notes
- 确定性优先：示例执行体不含 LLM；一旦启用 LLM，必须在 closure 中登记模型摘要
- `seal` 默认 `digest-only`，不冒充密码学签名

## [0.2.0] - 规范草案

- 来自《工作智能体（WorkAgent）定制开发指南（兼容规范 v0.2）》
