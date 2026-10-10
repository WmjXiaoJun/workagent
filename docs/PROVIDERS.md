# Model adapters: 22 providers, zero third-party dependencies

```bash
workagent models            # list every provider, endpoint, required env var and availability (no network)
workagent models --probe    # additionally probe whether local endpoints are running
```

## Three protocol families

The adapter layer implements **three protocols**; everything else is covered by one engine plus a
profile table:

| Family | Covers | Request | Auth |
|---|---|---|---|
| **OpenAI-compatible** | the vast majority (every Chinese vendor, all local inference) | `POST /chat/completions` | `Authorization: Bearer <key>` |
| **Anthropic** | Claude | `POST /messages` | `x-api-key` + `anthropic-version` |
| **Gemini** | Google | `POST /models/<m>:generateContent?key=` | URL parameter |

## Provider list

### Local inference (no key, offline, can reach L2/L3)
| Provider | Endpoint | Default model |
|---|---|---|
| `ollama` | `http://127.0.0.1:11434/v1` | `qwen2.5:7b` |
| `vllm` | `http://127.0.0.1:8000/v1` | must be specified |
| `lmstudio` | `http://127.0.0.1:1234/v1` | must be specified |

### International
| Provider | Env var | Default model |
|---|---|---|
| `openai` | `OPENAI_API_KEY` | gpt-4o-mini |
| `anthropic` | `ANTHROPIC_API_KEY` | claude-sonnet-4-5 |
| `gemini` | `GEMINI_API_KEY` | gemini-2.0-flash |
| `mistral` | `MISTRAL_API_KEY` | mistral-large-latest |
| `xai` | `XAI_API_KEY` | grok-2-latest |
| `groq` | `GROQ_API_KEY` | llama-3.3-70b-versatile |
| `together` | `TOGETHER_API_KEY` | Llama-3.3-70B-Instruct-Turbo |
| `openrouter` | `OPENROUTER_API_KEY` | (aggregates hundreds of models) |

### Chinese vendors
| Provider | Env var | Default model |
|---|---|---|
| `deepseek` | `DEEPSEEK_API_KEY` | deepseek-chat |
| `qwen` (Tongyi / Bailian) | `DASHSCOPE_API_KEY` | qwen-plus |
| `moonshot` (Kimi) | `MOONSHOT_API_KEY` | moonshot-v1-8k |
| `zhipu` (GLM) | `ZHIPUAI_API_KEY` | glm-4-plus |
| `doubao` (Volcano Ark) | `ARK_API_KEY` | must be an inference endpoint id |
| `hunyuan` (Tencent) | `HUNYUAN_API_KEY` | hunyuan-turbo |
| `baichuan` | `BAICHUAN_API_KEY` | Baichuan4 |
| `minimax` | `MINIMAX_API_KEY` | abab6.5s-chat |
| `spark` (iFlytek) | `SPARK_API_KEY` | generalv3.5 |
| `siliconflow` | `SILICONFLOW_API_KEY` | Qwen/Qwen2.5-72B-Instruct |

### Anything else
Any OpenAI-compatible endpoint:

```bash
export CUSTOM_BASE_URL=https://your-endpoint/v1
export CUSTOM_API_KEY=sk-xxx
workagent run --case bid_quote --input examples/bid_quote/input.natural.json . --provider custom --model your-model
```

## Switching models: one command

```bash
# local qwen2.5:7b (default, works offline)
workagent run --case expense_audit --input examples/expense_audit/input.natural.json .

# switch to DeepSeek
export DEEPSEEK_API_KEY=sk-xxx
workagent run --case expense_audit --input examples/expense_audit/input.natural.json . --provider deepseek --model deepseek-chat

# switch to Claude
export ANTHROPIC_API_KEY=sk-ant-xxx
workagent run --case expense_audit --input examples/expense_audit/input.natural.json . --provider anthropic --model claude-sonnet-4-5
```

**The executor does not change at all** — switching models swaps the *extractor*; not one line of the
rules or the judgement moves.

## The adapter layer is tested

```bash
go test ./internal/llm/ -v
```

Six tests cover the three protocol families: request headers, request body fields, response parsing.
They use `httptest` with a fake server, so they run in CI with **no API key and no network**.

```text
PASS: TestOpenAICompatibleChat    (Authorization / response_format / content parsing)
PASS: TestAnthropicChat           (x-api-key / system hoisting / content array / max_tokens required)
PASS: TestGeminiChat              (:generateContent / key parameter / candidates parsing)
PASS: TestUnknownProvider         (an unregistered provider must error)
PASS: TestLocalProviderNeedsNoKey (local inference requires no key)
PASS: TestProfileCount            (number of built-in profiles)
```

## Reproducibility level vs model choice

| The model you pick | The level you can reach |
|---|---|
| none (deterministic scenarios) | **L3** |
| local open weights + `temperature=0` + fixed seed | **L2** (measured: qwen2.5:7b returned byte-identical responses across three calls) |
| any cloud model | **L1** (replayed from a recording; the model itself is not guaranteed) |
| `temperature>0` | **L0** (a record of one call) |

The level is determined by `workagent` automatically and written into `evidence.json`.
**You are not allowed to declare it by hand.**
