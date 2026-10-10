# Changelog

本项目遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

## [0.13.0] - 2026-10-10

**12 个执行体全部迁移完成：执行体只输出 code + args，一个自然语言字都没有。**

### Changed
- 全部 12 个 `executor.py` 改为输出 `args` / `conclusion_args`；
  `messages.json` 扩充到 **85 条文案 / 12 张判定词表**，中英齐全。
- `--lang en` 现在对 12 个场景**全部生效**（此前只有 2 个）。

### Added
- `conclusion_code`：执行体按**事实**选文案码（如 `spc_control` 的"触发判异"与"过程受控"两种结论形态），
  文案仍由 `messages.json` 提供 —— "执行体不写人话"这条没有被打破。

### Fixed（迁移中发现的**同码不同文**，共 6 处）

文案表是 `code → 模板` 的映射，一个 code 有两种文案就会被静默渲染成其中一种。
这类错误**没有任何报错**，只会显示错的话：

| 场景 | 冲突的 code | 拆分为 |
|---|---|---|
| `audit_chain` | `AC-01`（断链 / 首次摄入带 old） | `AC-01` / `AC-01F` |
| `expense_audit` | `EXP-04`（日期倒挂 / 超时限） | `EXP-04` / `EXP-04B` |
| `point_table` | `PT-04`（地址非法 / 越界）、`PT-03`（倒置 / 非数值） | `PT-04` / `PT-04B`、`PT-03` / `PT-03B` |
| `pin_conflict` | `IRQ-02`（缺失 / 越界） | `IRQ-02` / `IRQ-02B` |
| `modbus_poll` | `POLL-SUGGEST`（提波特率 / 放宽周期） | `POLL-SUGGEST` / `POLL-SUGGEST-B` |
| `firmware_budget` | `FR-02`、`FR-04`（告警 / 正常） | 各拆为 `FR-0X` / `FR-0X-OK` |

### 顺带去掉两处"句子里塞执行体人话"
- `pin_conflict` 的结论原本嵌着 `errors[0]["message"]`（"首批：……"）—— 那是把执行体文案拼进句子，
  既语言相关又和 findings 重复。已去掉。
- `lab_report` 的结论有"有危急值才出现"的条件片段 —— 句内条件片段是语言相关的，改为常量格式。

### 兼容性
`findings[].message` 全部保留，`args` 为纯增量。归档因摘要变化已整体重生成（12 条）。

### 实测
12 个场景中英结论逐条对照通过；双语 `verify --all` 摘要序列逐位相同。

## [0.12.0] - 2026-10-10

**执行体不再写人话：只输出 code + args，文案集中到 `messages.json`。** 详见 `docs/MESSAGES.md`。

### Added
- `messages.json`：包级文案表，`zh` / `en` 并排。键是 `<caseID>.<CODE>`，
  `CONCLUSION` 为保留码。
- `internal/render`：按 `--lang` 把结构化事实渲染成人话；缺英文就**不渲染**、回落原文，不冒充已翻译。
- `check` 新增 **[6/6] 文案表**：有 zh 没 en 会**报错**，而不是等英文模式静默显示中文。
- `audit_chain` / `points_reconcile` 两个执行体已迁移为 code + args（其余 10 个待迁移）。

### 铁律：渲染绝不参与哈希

`output_digest` 取自执行体的**原始**结构化输出，渲染在取摘要**之后**。实测：

```text
$ workagent run --case audit_chain .            证据包编号 ed6532b74aec41d1
$ workagent run --case audit_chain . --lang en  Evidence bundle id ed6532b74aec41d1   ← 同一个
```

若渲染进了哈希，中文跑一次、英文跑一次会得到两个摘要，`verify` 直接失效。

### Fixed
- **同一个 code 不能有两种文案**：`audit_chain` 把「哈希链断裂」与「首次摄入却带 old_sha256」
  都记成 `AC-01`，后者会被渲染成前者的文案。已拆为 `AC-01` / `AC-01F`。
- 归档重建：两个场景的输出摘要因新增 `args` 而变化，旧记录已失效，故 12 条记录在 0.12.0 整体重生成。

### 兼容性
`findings[].message` **保留**（规范已有该字段），新增 `args`；无 `args` 时原样显示。
纯增量，不破坏老消费者。

## [0.11.0] - 2026-10-10

**CLI 双语：`--lang en` / `WORKAGENT_LANG=en`。**

### Added
- `internal/i18n`：用**内联中英对**（`i18n.T("中文", "English")`）而不是 key 目录。
  没有 key 就没有"拼错 key""忘了加翻译"这类错误，中文也留在调用点可见。
- `--lang` 开关可放在命令**之前或之后**；`WORKAGENT_LANG` 环境变量等效；默认中文。
- `agent.yaml` 新增 `title_en`（12 条）：标题是**内容**，真源在 manifest，
  不在 Go 里硬编码一张会漂移的映射表。老平台忽略该字段，属纯增量。
- 交付形态词表（测算/核验/抽取/排程/推演）映射放进 i18n —— 它是规范固定词汇。

### 实测
英文模式下逐命令扫描残留中文：

```text
usage 0 ｜ cases 0 ｜ check 0 ｜ models 0 ｜ attest 0 ｜ adapt 0 ｜ seal 0 ｜ verify --all 0
run 12   ← 全部来自 Python 执行体的 conclusion/findings
```

**Go CLI 已全部双语；Python 执行体尚未（下一步）。**

### Fixed
- **`agent.yaml` 版本再次漂移**：停在 0.8.0，而标签已到 v0.10.0。已同步到 0.11.0。
  这已是第二次 —— 说明"人肉记得同步版本"是不可靠的。
- `usage` 打印的版本改为**从 manifest 实时读**（`toolVersion` 常量长期写死 0.3.0）。
- `--lang` 一开始只解析 `os.Args[2:]`，导致 `workagent --lang en`（开关在命令位）
  静默失效。改为在**完整参数**上解析，并在分发前摘除。

## [0.9.0] - 2026-10-10

**`workagent record` —— 把结论变成可公开、可累积的资产。**

### Added
- `workagent record --case <ID>`：跑一次场景，把**结论 + 输入/输出/执行体摘要 + 证据包编号**
  归档到 `records/NNNN-<case>-<日期>.md`，并自动重建 `records/INDEX.md`
- `records/` 自带**覆盖全部 12 个场景的基线**：编号、判定、复现等级、输出摘要一览

### 设计要点
- **编号只增不改**：编号 = 已有最大值 + 1，记录从不重写
- **原始输入不进仓库**，只留 sha256 —— 这正是记录可以公开的原因：
  它证明"当时得出了什么结论"并让任何人复现，却不泄露数据本身
- **`INDEX.md` 是生成物**，由 `records/*.md` 重建，手改会丢
- 带模型的场景**默认走重放**：公开记录必须是谁都能离线复现的

### Fixed（都是这一版自己踩出来的）
- `record` 第一版默认**真实调用模型**，三个 L2/L1 场景因没有 API key 直接记录失败。
  这违背了记录的意义 —— 改为默认重放，`--live` 才真实调用。
- `run` 省略 `--input` 时只看 `sample_input`，
  而只声明 `sample_natural` 的场景（三个带模型抽取的）会直接报错。改为两级回退。

## [0.8.0] - 2026-10-10

**用真实生产数据反推出两个核验型场景；新增 `workagent mcp` 与 DSH 插件。**

### Added
- `workagent mcp`：把交付包本身暴露成 **MCP server**（stdio）。
  任何 MCP 客户端都能把「跑场景 → 拿可复现结论」当工具调用。
  实现直接起自身子进程并捕获输出，保证 MCP 与命令行**走同一份逻辑**。
- `dsh-plugin/`：DeepSeek Harness 插件 bundle，5 行配置把上面的 MCP server 接进 Harness。
- `points_reconcile` **积分账本一致性对账**（核验型，L3）—— 七项不变量
- `audit_chain` **知识库摄入审计核验**（核验型，L3）—— 六项校验

### 这两个场景怎么来的
规则**先在真实生产库上验证**（违反数 0）再固化；示例数据用合成行，结构照抄真库。
真实数据（用户、金额、会话令牌哈希、文档路径）**不入公开仓库**。

### Fixed
- **`agent.yaml` 版本漂移**：manifest 停在 0.6.2，而标签已到 0.7.x。已同步。
- MCP `serverInfo.version` 改为**从 manifest 实时读**，不再用会漂的常量
  （`toolVersion` 常量此前写死 0.3.0，漂了很久没人发现）。
- `audit_chain` 第一版**按 ts 全局排序**，把"时间戳倒退"这个要检测的现象直接排没了。
  改为"按文档分组、组内保留日志原始顺序"。
- `points_reconcile` 第一版不认识 `sub2api_reserve`，产生假断链。

### 现状
| 形态 | 场景数 |
|---|---|
| 测算型 | 5 |
| 核验型 | **7** |
| 抽取型 / 排程型 / 推演型 | 待补 |

## [0.7.0] - 2026-10-10

**补齐"测算型"：新增两个工程现场每天都在算的场景。**

### Added
- `modbus_poll` **通信负载与轮询周期测算**（测算型，L3）
  - 规则来自 Modbus 帧格式：请求 8 字节、响应 5+2n 字节、字符位数、帧间静默 3.5 字符时间、单次上限 125 寄存器
  - **相邻寄存器自动合并**：示例 12 个点合并后 5 次请求，一轮 75 ms（目标 500 ms）
  - 输出每次请求的功能码 / 起始地址 / 寄存器数 / 耗时，以及距离目标周期的余量与优化建议
- `firmware_budget` **固件资源预算测算**（测算型，L3）
  - Flash / RAM 占用汇总与容量比对，占用最大的模块排序
  - **任务栈余量检查（FR-05/FR-06）**：栈溢出是现场最难查的故障，余量不足直接报错
  - 示例抓出「通信任务栈余量仅 16%，低于 20% 下限」

### 形态分布
| 形态 | 场景数 |
|---|---|
| 测算型 | **5** |
| 核验型 | 5 |
| 抽取型 / 排程型 / 推演型 | 待补 |

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
