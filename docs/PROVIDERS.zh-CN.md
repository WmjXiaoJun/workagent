# 模型适配：22 家服务商，零第三方依赖

[English](PROVIDERS.md) · **中文**

```bash
workagent models            # 列出全部服务商、端点、所需环境变量与可用状态（不联网）
workagent models --probe    # 额外探测本地端点是否在跑
```

## 三种协议族

适配层只实现了**三种协议**，其余全部通过"一个引擎 + 一张 profile 表"覆盖：

| 协议族 | 覆盖的服务商 | 请求 | 认证 |
|---|---|---|---|
| **OpenAI 兼容** | 绝大多数（含全部国内厂商与本地推理） | `POST /chat/completions` | `Authorization: Bearer <key>` |
| **Anthropic** | Claude | `POST /messages` | `x-api-key` + `anthropic-version` |
| **Gemini** | Google | `POST /models/<m>:generateContent?key=` | URL 参数 |

## 服务商清单

### 本地推理（无需密钥，可离线，可做到 L2/L3）
| Provider | 端点 | 默认模型 |
|---|---|---|
| `ollama` | `http://127.0.0.1:11434/v1` | `qwen2.5:7b` |
| `vllm` | `http://127.0.0.1:8000/v1` | 需指定 |
| `lmstudio` | `http://127.0.0.1:1234/v1` | 需指定 |

### 国际
| Provider | 环境变量 | 默认模型 |
|---|---|---|
| `openai` | `OPENAI_API_KEY` | gpt-4o-mini |
| `anthropic` | `ANTHROPIC_API_KEY` | claude-sonnet-4-5 |
| `gemini` | `GEMINI_API_KEY` | gemini-2.0-flash |
| `mistral` | `MISTRAL_API_KEY` | mistral-large-latest |
| `xai` | `XAI_API_KEY` | grok-2-latest |
| `groq` | `GROQ_API_KEY` | llama-3.3-70b-versatile |
| `together` | `TOGETHER_API_KEY` | Llama-3.3-70B-Instruct-Turbo |
| `openrouter` | `OPENROUTER_API_KEY` | （聚合数百模型） |

### 国内
| Provider | 环境变量 | 默认模型 |
|---|---|---|
| `deepseek` | `DEEPSEEK_API_KEY` | deepseek-chat |
| `qwen`（通义/百炼） | `DASHSCOPE_API_KEY` | qwen-plus |
| `moonshot`（Kimi） | `MOONSHOT_API_KEY` | moonshot-v1-8k |
| `zhipu`（智谱 GLM） | `ZHIPUAI_API_KEY` | glm-4-plus |
| `doubao`（火山方舟） | `ARK_API_KEY` | 需填推理接入点 ID |
| `hunyuan`（腾讯混元） | `HUNYUAN_API_KEY` | hunyuan-turbo |
| `baichuan` | `BAICHUAN_API_KEY` | Baichuan4 |
| `minimax` | `MINIMAX_API_KEY` | abab6.5s-chat |
| `spark`（讯飞星火） | `SPARK_API_KEY` | generalv3.5 |
| `siliconflow`（硅基流动） | `SILICONFLOW_API_KEY` | Qwen/Qwen2.5-72B-Instruct |

### 任意其它
任何 OpenAI 兼容端点：

```bash
export CUSTOM_BASE_URL=https://your-endpoint/v1
export CUSTOM_API_KEY=sk-xxx
workagent run --case bid_quote --input examples/bid_quote/input.natural.json . --provider custom --model your-model
```

## 切换模型：一条命令

```bash
# 本地 qwen2.5:7b（默认，可离线）
workagent run --case expense_audit --input examples/expense_audit/input.natural.json .

# 换成 DeepSeek
export DEEPSEEK_API_KEY=sk-xxx
workagent run --case expense_audit --input examples/expense_audit/input.natural.json . --provider deepseek --model deepseek-chat

# 换成 Claude
export ANTHROPIC_API_KEY=sk-ant-xxx
workagent run --case expense_audit --input examples/expense_audit/input.natural.json . --provider anthropic --model claude-sonnet-4-5
```

**执行体完全不用改** —— 换模型只是换了一个"抽取器"，规则与判定一行都不动。

## 适配层是被测试过的

```bash
go test ./internal/llm/ -v
```

6 个测试覆盖三种协议族：请求头、请求体字段、响应解析。
用 `httptest` 造假服务端，**不需要任何 API key、不联网**也能在 CI 里跑。

```text
PASS: TestOpenAICompatibleChat   （Authorization / response_format / content 解析）
PASS: TestAnthropicChat          （x-api-key / system 提升 / content 数组解析 / max_tokens 必填）
PASS: TestGeminiChat             （:generateContent / key 参数 / candidates 解析）
PASS: TestUnknownProvider        （未注册 provider 必须报错）
PASS: TestLocalProviderNeedsNoKey（本地推理不要求密钥）
PASS: TestProfileCount           （内置 profile 数量）
```

## 复现等级与模型选择

| 你选的模型 | 能达到的等级 |
|---|---|
| 不用模型（纯确定性场景） | **L3** |
| 本地开权重 + `temperature=0` + 固定 seed | **L2**（实测 qwen2.5:7b 三次调用逐字节一致） |
| 任意云端模型 | **L1**（靠录制重放；模型本身不保证复现） |
| `temperature>0` | **L0**（只是一次调用记录） |

等级由 `workagent` 自动判定并写进 `evidence.json`，**不允许手工声称**。
