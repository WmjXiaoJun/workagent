<div align="center">

<img src="assets/logo.svg" width="112" height="112" alt="WorkAgent">

[English](README.md) · **中文**

# WorkAgent

### 别人交付对话，我们交付可复现的结论。

**A portable agent package where every conclusion is reproducible, traceable, and signable.**

![spec](https://img.shields.io/badge/spec-v0.6.0-blue)
![cases](https://img.shields.io/badge/scenarios-12-orange)
![runtime deps](https://img.shields.io/badge/runtime%20deps-0-brightgreen)
![license](https://img.shields.io/badge/license-MIT-green)
![status](https://img.shields.io/badge/status-early-orange)
![go](https://img.shields.io/badge/go-1.21%2B-00ADD8)
![providers](https://img.shields.io/badge/LLM%20providers-22-blueviolet)

[30 秒上手](#30-秒上手) · [核心概念](#核心概念) · [兼容老平台](#兼容老平台这是本项目的重点) · [CLI](#九个命令) · [下载二进制](https://github.com/WmjXiaoJun/workagent/releases/latest) · [English](README.md)

</div>

---

## 这是什么

**一个能被验证的智能体交付包**：格式 + 零依赖工具链。

不是框架，不是平台，不是聊天机器人。就是一个文件夹——里面装着你的业务逻辑、你的隐含假设、你的失效边界、你的测试套件，和一个能证明"结论自生成后没有变过"的证据机制。

它只回答一个问题：

> **AI 给出的结论，凭什么信？**

今天的现实是：你能在 15 分钟里搭出一个 Agent，但你答不上来这三个问题——

| 问题 | 现状 | WorkAgent |
|---|---|---|
| 同样的输入，明天再跑一遍，结论还一样吗？ | 不知道 | `verify` 一键复现校验，摘要不一致直接 FAIL |
| 这个结论依赖了哪些假设？假设失效会怎样？ | 散落在脑子里 | `provenance/assumptions.json` 强制显式固化 |
| 什么情况下这个结论就不能用了？ | 没有定义 | `boundary.json` + `stop_condition`：越界**拒绝执行**，而不是硬算 |
| 别人怎么确认你给我的包没被改过？ | 无法确认 | `seal` 全包摘要 + 文件级哈希清单 |
| 在只支持老标准的平台上跑，会丢什么？ | 说不清 | `compat.json` **显式声明降级行为** |

---

## 30 秒上手

**零第三方依赖。Go 1.21+。编译出一个静态二进制，对方不需要装任何运行时。**

```bash
git clone https://github.com/WmjXiaoJun/workagent.git
cd workagent
go build -o workagent ./cmd/workagent

./workagent check .                                          # ① 结构与兼容声明校验
./workagent run   --input examples/bid_quote/input.json .    # ② 执行，产出证据包
./workagent verify .                                         # ③ 复现校验（关键一步）
```

三平台交叉编译（`make release` 一次产出五个）：

```bash
GOOS=linux   GOARCH=amd64 go build -o dist/workagent-linux-amd64     ./cmd/workagent
GOOS=darwin  GOARCH=arm64 go build -o dist/workagent-darwin-arm64    ./cmd/workagent
GOOS=windows GOARCH=amd64 go build -o dist/workagent-windows-amd64.exe ./cmd/workagent
```

真实运行输出（不是示意，是仓库里跑出来的）：

```text
$ ./workagent run --input examples/bid_quote/input.json .
run —— 执行完成
  [OK]   证据包编号 6723c555d11c0a3e
  [OK]   结论: 建议报价 CNY 1,229,437.44（直接成本 968,000.00，利润率 12%）
  [OK]   输出摘要 8e38afbabb710b00
  [--]   证据目录 .../out/evidence/6723c555d11c0a3e

$ ./workagent verify .
verify —— 复现校验，证据包 6723c555d11c0a3e
  期望摘要 8e38afbabb710b001ed63b6fbdd7a22d
  重放摘要 8e38afbabb710b001ed63b6fbdd7a22d
结果: PASS —— 同输入产出同结论，可复现。
```

越界输入不会被硬算，而是被拒绝：

```text
$ echo '{"project":"x","line_items":[]}' > bad.json
$ ./workagent run --input bad.json .
执行体失败 (exit 2): BOUNDARY_VIOLATION: line_items 为空或不是数组
```

---

## 九个命令

| 命令 | 做什么 | 为什么存在 |
|---|---|---|
| `models` | 列出 22 家内置模型服务商、端点、所需环境变量与可用状态 | 不用记各家的 API 格式，有 key 就能跑 |
| `check` | 校验包结构、JSON 可解析性、兼容声明、边界与测试套件 | 交付前最后一道门 |
| `adapt` | 从 `agent.yaml` 生成 `adapters/`（Agent Plugins 1.0 `plugin.json` + A2A `agent-card.json`） | 一次编写，多生态可用，且不污染真源 |
| `attest` | 计算 **Attestation Closure** 摘要 | 让"可复现"有可核对的证据，而不是一句承诺 |
| `run` | 执行执行体，产出证据包 + 摘要 + 溯源记录 | 结论必须带着证据一起走 |
| `verify` | 重放并比对结论摘要，PASS / FAIL | **本项目的核心**。不可复现 = 不可交付 |
| `record` | 跑一次场景，把结论 + 证据摘要归档到 `records/` | **唯一会随时间增值的资产** |
| `mcp` | 把交付包本身作为 **MCP server** 提供（stdio） | 任何 MCP 客户端都能把"跑场景拿可复现结论"当工具调用 |
| `seal` | 生成全包文件级摘要 `signature/agent.sig` | 证明内容自生成后未变 |

### 分发：先走 MCP

```bash
workagent mcp .          # stdio MCP server，不需要端口，也不需要常驻进程
```

实现方式是**起自身子进程并捕获输出**，所以 **MCP 调用与命令行调用走的是同一份逻辑**——
不存在第二条会漂移的代码路径。

`dsh-plugin/` 是一个 DeepSeek Harness 插件 bundle：五行配置把这个 MCP server 接进 Harness。
它同样适用于任何 MCP 客户端，不只 DSH。

> ⚠️ **MCP server 侧已验证**（握手、`tools/list`、`tools/call`、未知工具报错都实测过）；
> **Cordis 装载侧未验证** —— 原因见 `dsh-plugin/README.md`。

### 公开记录 —— 唯一会增值的资产

```bash
workagent record --case points_reconcile .
# → records/0013-points_reconcile-20261010.md
# → records/INDEX.md（自动重建）
```

每一次结论都带着摘要归档，编号、**只增不改、从不重写**。

**原始输入不进仓库，只进它的 sha256。** 这正是记录可以公开的原因：
它证明"当时得出了什么结论"，并让任何人都能复现，却不泄露数据本身。

`records/` 自带覆盖全部场景的 12 条基线。**半年后回来跑一次 `verify --all` 与这些摘要比对**，
"可复现"就从一句承诺变成了一个可核对的事实。

---

## 模型适配与复现等级

**22 家内置服务商，零第三方依赖。** `workagent models` 一条命令看清全部：

```text
$ workagent models
PROVIDER     PROTOCOL   端点                                 密钥环境变量              状态
ollama       openai     http://127.0.0.1:11434/v1           (无需)                    本地可用
openai       openai     https://api.openai.com/v1            OPENAI_API_KEY            缺少密钥
anthropic    anthropic  https://api.anthropic.com/v1         ANTHROPIC_API_KEY         缺少密钥
deepseek     openai     https://api.deepseek.com/v1          DEEPSEEK_API_KEY          缺少密钥
qwen         openai     https://dashscope.aliyuncs.com/...   DASHSCOPE_API_KEY         缺少密钥
...（共 22 家）
```

覆盖 OpenAI / Anthropic / Gemini / Mistral / xAI / Groq / Together / OpenRouter，
国内 DeepSeek / 通义千问 / Kimi / 智谱 GLM / 豆包 / 混元 / 百川 / MiniMax / 星火 / 硅基流动，
以及本地 Ollama / vLLM / LM Studio。**任何 OpenAI 兼容端点**用 `custom` 就能接入。

### 但模型不可复现，怎么办

这是本项目的核心矛盾。解法是**把一次 LLM 调用拆成两半**：

```text
非结构化输入 ──[模型抽取]──▶ 结构化中间表示 ──[确定性代码]──▶ 结论 + 证据
                     ↑ 不可复现 → 录制成档案         ↑ 完全可复现
```

**模型只做翻译，判断交给代码。** 然后：

| 命令 | 行为 |
|---|---|
| `run` | 真实调用模型，把**原始响应**录进 `provenance/llm/` |
| `run --replay` | 用录制响应，**离线**跑通，不需要 key、不需要网络 |
| `verify` | 重放录制，校验裁决过程 —— CI 里跑的就是这条 |
| `verify --live` | **重新真实调用模型**，测量它到底复现不复现 |

### 复现等级（如实标注，绝不冒充）

| 等级 | 含义 | 触发条件 |
|---|---|---|
| **L3** | 完全可复现 | 纯确定性，无 LLM |
| **L2** | 决策可复现 | 本地开权重 + `temperature=0` + 固定 seed → 实测逐字节一致 |
| **L1** | 记录可复现 | 云端模型本身不可复现，但裁决过程可重放 |
| **L0** | 不可复现 | `temperature>0`，只是一次调用记录 |

等级会写进 `evidence.json`。**我们不把 L1 说成 L2。**

> **实测**：本地 `ollama / qwen2.5:7b`、`temperature=0`、`seed=42`，
> 三次独立调用返回**逐字节相同**的响应（`response_digest 4d6abc79490d3241…`）→ 该路径达到 **L2**。
> 这是测出来的，不是声称的。

---

## 十二个可验证场景，按交付形态分

**不按行业分，按交付形态分。** 行业是结果不是依据——为什么这样切见 `docs/VERTICALS.md`。
一个包承载多个场景，**共用同一套基础设施**（兼容声明、溯源、证据包、复现校验）。

```text
$ workagent cases .
WorkAgent cases —— 按交付形态分组

【测算型】5 个
  bid_quote       L2/L1    投标报价测算
  salary_tax      L3       工资薪金个税测算
  loan_dscr       L3       项目贷款偿债能力测算
  modbus_poll     L3       通信负载与轮询周期测算
  firmware_budget L3       固件资源预算测算

【核验型】7 个
  expense_audit    L2/L1    报销单合规审核
  lab_report       L2/L1    检验报告异常判定
  point_table      L2/L1    通信点表核对
  spc_control      L3       SPC 控制图判异
  pin_conflict     L3       引脚 / 中断 / DMA 资源冲突检测
  points_reconcile L3       积分账本一致性对账
  audit_chain      L3       知识库摄入审计核验
```

```bash
workagent run --case spc_control --input examples/spc_control/input.json .
workagent verify --all .
```

| 场景 | 做什么 | 规则来源 |
|---|---|---|
| `salary_tax` | 累计预扣法逐月算个税 + 实际税负率 | 《个人所得税法》7 级累进预扣率表 |
| `loan_dscr` | DSCR 与利息保障倍数，阈值**写在输入里**不写死在代码里 | DSCR 标准公式 |
| `spc_control` | 8 条判异准则，指出**哪一点违反哪条** | Western Electric / Nelson |
| `pin_conflict` | 引脚复用 / 中断优先级 / DMA 通道冲突 | MCU 资源一致性规则集 |
| `bid_quote` | 结构化或自然语言两种入口，共用同一执行体 | 工程造价惯例 |
| `expense_audit` | 单笔限额、类别限额、时限、发票号、合计一致性 | 企业报销制度（示例阈值） |
| `lab_report` | 按性别区分的参考区间 + 危急值 | 成人常见参考区间（示例值） |
| `point_table` | 地址重叠、数据类型、量程、越界 + **Modbus 参考地址归一化** | Modbus 寻址规则 |

**四个 L3 场景完全不依赖模型**——同样输入，任何机器任何时间，结论摘要都一样。

> 详细说明见 `docs/EXAMPLES.md` ｜ 模型适配见 `docs/PROVIDERS.md`。
> ⚠️ 医疗场景的参考区间为示例值，**不构成医学诊断**。

---

## 核心概念

### 1. 真源与适配层

`agent.yaml` 是**唯一真源**。`adapters/` 永远是**生成物**。

```text
agent.yaml  ──adapt──▶  adapters/plugin.json      (Agent Plugins 1.0 客户端)
   │        ──adapt──▶  adapters/agent-card.json  (A2A 客户端发现)
   └── 永远不生成、不覆盖、不修改
```

改适配层等于白改，下次生成会被覆盖。**一个真源，多个生态，零同步成本。**

### 2. 证据包

每次 `run` 产出一个带编号的证据目录：

```text
out/evidence/6723c555d11c0a3e/
├── input.json      输入快照
├── output.json     标准化结论
├── summary.md      人工可读、可签字、可归档的摘要
└── evidence.json   摘要、执行体、版本、时间
```

结论和证据是**同一个东西的两面**——引用结论时必须同时引用 `evidence_id`。

### 3. 复现校验（核心）

`verify` 会拿原始输入**重新执行一遍**，比对结论摘要。

- 一致 → `PASS`，结论可交付。
- 不一致 → `FAIL`，**结论禁止对外使用**。

这让"可复现"从一句形容词变成一个**可执行的门禁**——CI 里跑不过就不许合并。

### 4. Attestation Closure（计算闭包）

只写一个 `Dockerfile` 不足以证明"结论可复现"。`attest/closure.json` 记录覆盖推理函数**完整计算闭包**的摘要：

```text
executor_digest · inputs_schema_digest · workflow_digest · manifest_digest
python_version  · platform · numeric_env
model_weights_digest · tokenizer_digest · chat_template_digest
engine_build_digest  · decoding_config
```

> 原则：**任何落在被证明集合之外的组件，都是一条伪造通道。**
>
> 本项目示例是纯确定性执行，不含 LLM；一旦你启用 LLM 步骤，上述模型字段**必须**填写，否则不允许声称"结论可复现"。

### 5. 边界即代码

`boundary.json` 里的 `invalid_when` 不是文档，是**运行时会触发拒绝的条件**。命中即报错退出，绝不输出结论。

---

## 兼容老平台（这是本项目的重点）

很多团队的真实约束是：**新能力要上，但老平台不能坏。**

WorkAgent 的策略只有一句话：**新增只加目录，不改字段。**

`x-workagent/` 是扩展区。只支持 APS 基础字段的老平台读到它会**直接忽略**，包照样能跑、照样出结论。

但"照样能跑"和"没损失"是两件事。所以我们把损失**显式写进 `compat.json`**：

| 能力 | 老平台上 |
|---|---|
| 读取清单、输入输出校验、工作流执行、输出 `summary.md` | **保留** |
| 失效边界校验 | **丢失** |
| 回归基准比对 | **丢失** |
| 结论可复现校验 | **丢失** |
| 包完整性验签 | **丢失** |

并且带一句明确声明：

> 在仅支持 APS 基础字段的平台上，本包可以运行并产出结论；但该结论**不可复现校验、不可验签、不受失效边界保护**。请勿据此结论做高风险决策。

**大多数项目不说这句话。我们认为说出来才是专业。**

---

## 包结构

```text
workagent/
├── agent.yaml                     # ★ 唯一真源（APS 基础字段 + x-workagent 扩展区）
├── compat.json                    # ★ 兼容降级声明
├── inputs.schema.json             # 输入标准化校验
├── execution/
│   ├── workflow.yml               # 工作流编排（确定性优先）
│   └── runtime.dockerfile         # 环境固化
├── artifacts/
│   ├── summary.md                 # 人工可读摘要（生成）
│   ├── summary.template.md        # 摘要模板
│   └── output_schema.json         # 输出标准化定义
├── provenance/                    # 溯源层
│   ├── trace.json                 #   执行记录（verify 据此重放）
│   ├── data_sources.json          #   数据源版本与哈希
│   └── assumptions.json           #   隐含假设 + 排除场景
├── attest/closure.json            # ★ Attestation Closure 计算闭包摘要
├── x-workagent/                   # ★ 扩展层（老平台忽略它）
│   ├── boundary.json              #   失效边界 + 置信度 + stop_condition
│   └── test_suite/                #   unit / regression / sensitivity / stress
├── adapters/                      # 生成物：plugin.json + agent-card.json
├── signature/agent.sig            # 全包摘要
├── cmd/workagent/                 # 零依赖 CLI（Go）
│   ├── main.go
│   └── yaml.go
├── go.mod                         # 零第三方依赖
├── Makefile                       # make release 一次产出五平台二进制
└── examples/bid_quote/            # 可运行示例
```

---

## 设计原则

**三个"只"**——这是给个人开发者的，不是给平台的：

1. **只做一个细分场景。** 通用助手你要跟平台比参数，必输；窄场景里你比的是领域规则，平台进不来。
2. **只做能被验真的结论。** 这是唯一的护城河。选领域有个硬标准：**结论事后能被证真或证伪**。
3. **只做敢署名的交付。** 真人署名 + 可追溯，是"全民 agent"时代最稀缺的东西。

**三条工程铁律：**

1. **确定性优先。** 能写成代码的，不要交给模型。LLM 只放在真正需要判断的关节上，并登记闭包摘要。
2. **只加不改。** 兼容性靠新增目录，不靠修改已有字段。
3. **诚实原则。** 没有私钥就只出摘要，不写"已签名"；不确定就写"不确定"。`seal` 的 `scheme` 字段会如实标明 `digest-only`。

---

## 为什么不用别的

| 你可能想问 | 回答 |
|---|---|
| 为什么不用 LangGraph / CrewAI / AutoGen？ | 它们是**运行时**，WorkAgent 是**交付物格式**。不冲突——你用它们跑，用 WorkAgent 交付。 |
| 为什么不直接用 Agent Plugins 1.0？ | 它管**打包分发**，不管可复现。本项目 `adapt` 会生成合规的 `plugin.json`，你两边都占。 |
| 为什么不直接用 Docker？ | Docker 固化**环境**，不固化**结论**。两者互补：`runtime.dockerfile` + `attest/closure.json`。 |
| 为什么不用 A2A / MCP？ | 它们管**互操作**。WorkAgent 生成 A2A `agent-card.json`，可直接被 A2A 生态发现。 |
| 为什么不接那些"身份 / 溯源"标准？ | 那些是企业级基础设施（KYA、WIMSE、AP2…），个人接不动也不需要。我们把其中**个人做得到的那一小块**（可复现 + 显式假设 + 边界 + 摘要）做扎实。 |

---

## 与现有标准的关系（诚实说明）

| 标准 | 关系 |
|---|---|
| APS 基础字段结构 | **兼容**：`agent.yaml` 保留原字段结构不动 |
| Agent Plugins 1.0（Vercel 牵头，OpenAI/AWS/GitHub/Microsoft 支持） | **适配**：`adapt` 生成其 `plugin.json`；本规范**不声称遵循**其全部要求 |
| A2A（Agent2Agent，现由 AAIF 托管） | **适配**：`adapt` 生成 `agent-card.json` |
| IETF AAT（`draft-sharif-agent-audit-trail`） | **参考**：`trace.json` 字段设计参考其思路，不声称遵循 |
| IETF GAR（`draft-sato-soos-gar`） | **参考**：审计包思路参考，不声称遵循 |
| OWASP AISVS C9.4 | **参考**：边界与溯源设计参考其要求，不声称合规 |

> ⚠️ 以上 GAR / AAT **均为个人 Internet-Draft，无 IETF 正式地位**；AAIF 是托管基金会，非协议本身。本项目不冒用任何标准的名义。

---

## Roadmap

- [x] **v0.3** 个人可验证版：真源 + 兼容声明 + 计算闭包 + 复现校验 + 适配层
- [ ] **v0.4** `--sign`：接入 GPG / Ed25519 真签名（取代 digest-only）
- [ ] **v0.4** JSON Schema 真实校验（当前为结构声明，未做运行时校验）
- [ ] **v0.5** 测试套件执行器：现有 unit/regression/sensitivity/stress 仅为声明，需要跑起来
- [ ] **v0.5** 公开记录页：把每个证据包编号变成可查的历史
- [ ] **v0.6** 多 agent 编排的全局一致性校验（防"每个都对、整体错"）
- [ ] **v1.0** 冻结规范

---

## FAQ

**Q：这跟"写个 README 说我的 AI 很准"有什么区别？**
A：区别是 `verify` 会 FAIL。形容词不会。

**Q：我的场景用了 LLM，还能用吗？**
A：能。但你要么接受"结论不可复现"（那就诚实标注），要么登记完整的模型闭包摘要（`attest/closure.json` 已经预留字段）。没有第三条路。

**Q：为什么用 Go，不用 Python / Rust？**
A：Go 编译出**单个静态二进制**，还能秒级交叉编译三平台——收到包的人不装任何运行时就能验证。Rust 同样能，但要引入 `serde@@ / `sha2@@ 等 crate，**违背"零第三方依赖"这条铁律**；Python 的问题不在语言，而在分发（对方没装 Python 就用不了）。
**执行体**可以是任何语言——CLI 只要求它遵守契约：读 `argv[1]`，向 stdout 输出 `{"output": {...}}`。

**Q：适合什么场景？**
A：需要**逻辑论证、数据推演、结果核验**的场景：工程测算、量化预测、科研论证、项目评估、风控审查。不适合纯文案、通知、闲聊。

**Q：为什么叫 WorkAgent？**
A：因为它交付的是**工作成果**，不是对话。

---

## 参与

最有价值的贡献：**一个真实场景的执行体**，或**一个"结论不可复现"的反例**。详见 [CONTRIBUTING.md](CONTRIBUTING.md)。

`verify` 跑不过的 PR 不会被合并。这不是苛刻，这就是本项目存在的意义。

---

## 许可与免责

[MIT](LICENSE) © 2026 WorkAgent contributors。

**免责**：本项目提供的是**可验证性的工具与格式**，不是业务结论。任何由本工具产出的结论，其正确性由该包的作者与签字人负责。请务必阅读每个包的 `x-workagent/boundary.json` 与 `compat.json`。
