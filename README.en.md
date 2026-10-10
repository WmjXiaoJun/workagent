<div align="center">

<img src="assets/logo.svg" width="112" height="112" alt="WorkAgent">

# WorkAgent

### Others ship conversations. We ship reproducible conclusions.

**A portable agent package where every conclusion is reproducible, traceable, and signable.**

![spec](https://img.shields.io/badge/spec-v0.6.0-blue)
![cases](https://img.shields.io/badge/scenarios-8-orange)
![runtime deps](https://img.shields.io/badge/runtime%20deps-0-brightgreen)
![license](https://img.shields.io/badge/license-MIT-green)
![status](https://img.shields.io/badge/status-early-orange)
![go](https://img.shields.io/badge/go-1.21%2B-00ADD8)
![providers](https://img.shields.io/badge/LLM%20providers-22-blueviolet)

[Quickstart](#quickstart) · [Concepts](#core-concepts) · [Legacy compatibility](#legacy-platform-compatibility) · [CLI](#the-seven-commands) · [Download binaries](https://github.com/WmjXiaoJun/workagent/releases/latest) · [中文](README.md)

</div>

---

## What is this

**A verifiable agent delivery package**: a format, plus a zero-dependency toolchain.

Not a framework. Not a platform. Not a chatbot. It is a folder — carrying your business logic, your hidden assumptions, your failure boundaries, your test suite, and an evidence mechanism that proves *the conclusion has not changed since it was produced*.

It answers exactly one question:

> **Why should anyone trust a conclusion produced by an AI?**

The reality today: you can build an agent in 15 minutes, but you cannot answer these three questions —

| Question | Today | WorkAgent |
|---|---|---|
| Same input, run it again tomorrow — same conclusion? | Unknown | `verify` re-runs and compares digests; mismatch = FAIL |
| Which assumptions does this conclusion rest on? What if they break? | In someone's head | `provenance/assumptions.json`, explicitly recorded |
| When is this conclusion no longer valid? | Undefined | `boundary.json` + `stop_condition`: out-of-range input is **rejected**, not force-computed |
| How do I know the package you sent me is unmodified? | No way to know | `seal` produces a file-level digest manifest |
| What do I lose on an old platform? | Unclear | `compat.json` **declares the degradation explicitly** |

---

## Quickstart

**Zero third-party dependencies. Go 1.21+. One static binary — no runtime needed on the receiving end.**

```bash
git clone https://github.com/WmjXiaoJun/workagent.git
cd workagent
go build -o workagent ./cmd/workagent

./workagent check .                                          # 1. structure + compatibility check
./workagent run   --input examples/bid_quote/input.json .    # 2. execute, produce evidence bundle
./workagent verify .                                         # 3. reproducibility check (the important one)
```

Cross-compile for three platforms (or `make release` for all five):

```bash
GOOS=linux   GOARCH=amd64 go build -o dist/workagent-linux-amd64     ./cmd/workagent
GOOS=darwin  GOARCH=arm64 go build -o dist/workagent-darwin-arm64    ./cmd/workagent
GOOS=windows GOARCH=amd64 go build -o dist/workagent-windows-amd64.exe ./cmd/workagent
```

Real output — this is the repo actually running, not an illustration:

```text
$ ./workagent run --input examples/bid_quote/input.json .
run —— done
  [OK]   evidence id 6723c555d11c0a3e
  [OK]   conclusion: recommended price CNY 1,229,437.44 (direct cost 968,000.00, margin 12%)
  [OK]   output digest 8e38afbabb710b00
  [--]   evidence dir .../out/evidence/6723c555d11c0a3e

$ ./workagent verify .
verify —— replaying evidence bundle 6723c555d11c0a3e
  expected digest 8e38afbabb710b001ed63b6fbdd7a22d
  replay digest   8e38afbabb710b001ed63b6fbdd7a22d
result: PASS — same input, same conclusion. Reproducible.
```

An out-of-bounds input is rejected rather than silently computed:

```text
$ echo '{"project":"x","line_items":[]}' > bad.json
$ ./workagent run --input bad.json .
executor failed (exit 2): BOUNDARY_VIOLATION: line_items is empty or not an array
```

---

## The seven commands

| Command | What it does | Why it exists |
|---|---|---|
| `models` | Lists all 22 built-in providers, endpoints, required env keys and availability | No need to memorise each vendor's API shape |
| `check` | Validates package structure, JSON parseability, compatibility declaration, boundary and test suite | The last gate before delivery |
| `adapt` | Generates `adapters/` from `agent.yaml` (Agent Plugins 1.0 `plugin.json` + A2A `agent-card.json`) | One source, many ecosystems, no sync cost |
| `attest` | Computes the **Attestation Closure** digests | Makes "reproducible" checkable rather than a promise |
| `run` | Executes the agent, emits an evidence bundle, summary and trace | A conclusion must travel with its evidence |
| `verify` | Replays and compares conclusion digests; PASS / FAIL | **The core of this project.** Not reproducible = not deliverable |
| `seal` | Produces the whole-package digest `signature/agent.sig` | Proves content is unchanged since sealing |

---

## Model support and reproducibility levels

**22 built-in providers, zero third-party dependencies.** One command shows them all:

```text
$ workagent models
PROVIDER     PROTOCOL    endpoint                              env key                   status
ollama       openai     http://127.0.0.1:11434/v1             (none)                    local, ready
openai       openai     https://api.openai.com/v1             OPENAI_API_KEY            key missing
anthropic    anthropic  https://api.anthropic.com/v1          ANTHROPIC_API_KEY         key missing
deepseek     openai     https://api.deepseek.com/v1           DEEPSEEK_API_KEY          key missing
qwen         openai     https://dashscope.aliyuncs.com/...    DASHSCOPE_API_KEY         key missing
... (22 in total)
```

OpenAI / Anthropic / Gemini / Mistral / xAI / Groq / Together / OpenRouter,
plus DeepSeek / Qwen / Kimi / Zhipu GLM / Doubao / Hunyuan / Baichuan / MiniMax / Spark / SiliconFlow,
plus local Ollama / vLLM / LM Studio. **Any OpenAI-compatible endpoint** works via the `custom` profile.

### But models are not reproducible — so what

That is the central tension of this project. The resolution is to **split one LLM call into two halves**:

```text
unstructured input ──[model extracts]──▶ structured IR ──[deterministic code]──▶ conclusion + evidence
                          ^ not reproducible → recorded       ^ fully reproducible
```

**The model translates; the code decides.** Then:

| Command | Behaviour |
|---|---|
| `run` | Calls the model for real and records the **raw response** into `provenance/llm/` |
| `run --replay` | Uses the recorded response — **offline**, no key, no network |
| `verify` | Replays the record and checks the decision path — this is what CI runs |
| `verify --live` | **Calls the model again** to measure whether it actually reproduces |

### Reproducibility levels (labelled honestly, never inflated)

| Level | Meaning | Condition |
|---|---|---|
| **L3** | Fully reproducible | Pure determinism, no LLM |
| **L2** | Decision reproducible | Local open weights + `temperature=0` + fixed seed → byte-identical in measurement |
| **L1** | Record reproducible | Cloud model is not reproducible, but the decision path can be replayed |
| **L0** | Not reproducible | `temperature>0` — a call log, nothing more |

The level is written into `evidence.json`. **We never call L1 an L2.**

> **Measured**: local `ollama / qwen2.5:7b`, `temperature=0`, `seed=42` —
> three independent calls returned a **byte-identical** response (`response_digest 4d6abc79490d3241…`),
> so this path reaches **L2**. Measured, not claimed.

---

## Eight verifiable scenarios, grouped by delivery form

**Not by industry — by delivery form.** Industry is an outcome, not a criterion; see
`docs/VERTICALS.md` for why. One package carries many scenarios sharing the same
infrastructure (compatibility declaration, provenance, evidence bundles, reproducibility checks).

```text
$ workagent cases .
WorkAgent cases —— grouped by delivery form

[Estimation] 3
  bid_quote      L2/L1    Bid pricing
  salary_tax     L3       Payroll income tax
  loan_dscr      L3       Loan debt-service coverage

[Verification] 5
  expense_audit  L2/L1    Expense compliance
  lab_report     L2/L1    Lab result flagging
  point_table    L2/L1    Modbus point-table audit
  spc_control    L3       SPC control-chart rules
  pin_conflict   L3       Pin / IRQ / DMA resource conflicts
```

```bash
workagent run --case spc_control --input examples/spc_control/input.json .
workagent verify --all .
```

| Scenario | What it does | Rules from |
|---|---|---|
| `salary_tax` | Cumulative withholding, month by month, plus effective tax rate | PRC Individual Income Tax Law, 7 brackets |
| `loan_dscr` | DSCR and interest coverage; thresholds come from **input**, not hard-coded | Standard DSCR formula |
| `spc_control` | 8 rule violations, naming **which point broke which rule** | Western Electric / Nelson rules |
| `pin_conflict` | Pin muxing / IRQ priority / DMA channel conflicts | MCU resource consistency rules |
| `bid_quote` | Structured or natural-language input, same executor | Construction estimating practice |
| `expense_audit` | Per-item and per-category limits, deadlines, invoice numbers, totals | Corporate expense policy (sample) |
| `lab_report` | Sex-specific reference intervals plus critical values | Adult reference intervals (sample) |
| `point_table` | Address overlap, data types, ranges, out-of-bounds, **Modbus reference-address normalisation** | Modbus addressing rules |

**Four L3 scenarios do not use a model at all** — same input, same digest, on any machine at any time.

> See `docs/EXAMPLES.md` and `docs/PROVIDERS.md`.
> The healthcare reference intervals are samples and **do not constitute a diagnosis**.

---

## Core concepts

### 1. Single source of truth, plus adapters

`agent.yaml` is the **only** source of truth. `adapters/` is always **generated**.

```text
agent.yaml  ──adapt──▶  adapters/plugin.json      (Agent Plugins 1.0 clients)
   │        ──adapt──▶  adapters/agent-card.json  (A2A discovery)
   └── never generated, never overwritten, never modified
```

Editing the adapter layer is wasted work — the next generation overwrites it.

### 2. Evidence bundle

Every `run` produces a numbered evidence directory:

```text
out/evidence/6723c555d11c0a3e/
├── input.json      input snapshot
├── output.json     normalized conclusion
├── summary.md      human-readable, signable, archivable
└── evidence.json   digests, executor, versions, timestamps
```

A conclusion and its evidence are two faces of one thing — cite the `evidence_id` whenever you cite the conclusion.

### 3. Reproducibility check (the core)

`verify` **re-executes** from the original input and compares conclusion digests.

- Match → `PASS`. Deliverable.
- Mismatch → `FAIL`. **The conclusion must not be used.**

This turns "reproducible" from an adjective into an **executable gate** — if CI can't pass it, it doesn't merge.

### 4. Attestation Closure

A `Dockerfile` alone cannot prove reproducibility. `attest/closure.json` records digests covering the **complete computational closure** of the inference function:

```text
executor_digest · inputs_schema_digest · workflow_digest · manifest_digest
python_version  · platform · numeric_env
model_weights_digest · tokenizer_digest · chat_template_digest
engine_build_digest  · decoding_config
```

> Principle: **any component left outside the attested set is a forgery channel.**
>
> The bundled example is pure deterministic computation with no LLM. The moment you enable an LLM step, those model fields **must** be filled in — otherwise you are not allowed to claim the conclusion is reproducible.

### 5. Boundaries as code

`invalid_when` in `boundary.json` is not documentation — it is a **runtime rejection condition**. When hit, the agent exits with an error and never emits a conclusion.

---

## Legacy platform compatibility

Many teams face the same real constraint: **ship new capability without breaking the old platform.**

WorkAgent's entire strategy is one sentence: **add directories, never change fields.**

`x-workagent/` is the extension area. A legacy platform that only understands the base APS fields will **silently ignore it**, and the package still runs and still produces a conclusion.

But "still runs" and "nothing lost" are different things. So the loss is written explicitly into `compat.json`:

| Capability | On a legacy platform |
|---|---|
| Reading the manifest, input/output validation, workflow execution, `summary.md` | **preserved** |
| Failure-boundary enforcement | **lost** |
| Regression baseline comparison | **lost** |
| Reproducibility verification | **lost** |
| Package integrity verification | **lost** |

With an explicit statement attached:

> On a platform that supports only the base APS fields, this package still runs and produces a conclusion — but that conclusion **cannot be reproduced, cannot be verified, and is not boundary-checked**. Do not use it for high-stakes decisions.

**Most projects never say this. We think saying it is what professionalism looks like.**

---

## Repository layout

```text
workagent/
├── agent.yaml                     # single source of truth (base APS fields + x-workagent)
├── compat.json                    # explicit compatibility / degradation declaration
├── inputs.schema.json             # input validation
├── execution/
│   ├── workflow.yml               # workflow (deterministic first)
│   └── runtime.dockerfile         # frozen runtime
├── artifacts/
│   ├── summary.md                 # human-readable summary (generated)
│   ├── summary.template.md        # summary template
│   └── output_schema.json         # output contract
├── provenance/
│   ├── trace.json                 # execution record (verify replays from this)
│   ├── data_sources.json          # data source versions and hashes
│   └── assumptions.json           # explicit assumptions + exclusions
├── attest/closure.json            # Attestation Closure digests
├── x-workagent/                   # extension layer (ignored by legacy platforms)
│   ├── boundary.json              # validity boundary + confidence + stop_condition
│   └── test_suite/                # unit / regression / sensitivity / stress
├── adapters/                      # generated: plugin.json + agent-card.json
├── signature/agent.sig            # whole-package digest
├── cmd/workagent/                 # zero-dependency CLI (Go)
│   ├── main.go
│   └── yaml.go
├── go.mod                         # zero third-party dependencies
├── Makefile                       # make release builds all five binaries
└── examples/bid_quote/            # runnable example
```

---

## Design principles

**Three "only"s — written for individual developers, not for platforms:**

1. **Only one narrow scenario.** Competing on general capability means competing with platforms on parameters, and you lose. In a narrow scenario you compete on domain rules, where platforms cannot follow.
2. **Only verifiable conclusions.** This is the only moat. Choose a domain where the conclusion can be **proven right or wrong after the fact**.
3. **Only deliverables you would sign your name to.** A named human plus traceability is the scarcest thing in the age of ubiquitous agents.

**Three engineering rules:**

1. **Deterministic first.** Anything expressible as code should not be delegated to a model. Keep LLM calls at the joints that genuinely need judgement, and record their closure digests.
2. **Add, never change.** Compatibility comes from new directories, not from modifying existing fields.
3. **Honesty rule.** With no private key, emit a digest only — never write "signed". The `scheme` field in `seal` reports `digest-only` truthfully.

---

## Why not just use X

| You might ask | Answer |
|---|---|
| Why not LangGraph / CrewAI / AutoGen? | Those are **runtimes**; WorkAgent is a **delivery format**. Not in conflict — run on them, deliver with WorkAgent. |
| Why not just use Agent Plugins 1.0? | It covers **packaging and distribution**, not reproducibility. `adapt` generates a compliant `plugin.json`, so you get both. |
| Why not just use Docker? | Docker freezes the **environment**, not the **conclusion**. Complementary: `runtime.dockerfile` + `attest/closure.json`. |
| Why not A2A / MCP? | Those cover **interoperability**. WorkAgent emits an A2A `agent-card.json` and is discoverable by that ecosystem. |
| Why not adopt the enterprise identity/audit standards? | Those are enterprise infrastructure (KYA, WIMSE, AP2, …). An individual cannot and need not adopt them. We do the small slice an individual *can* do: reproducibility, explicit assumptions, boundaries, digests. |

---

## Relationship to existing standards (honest disclosure)

| Standard | Relationship |
|---|---|
| Base APS field structure | **Compatible**: field structure preserved in `agent.yaml` |
| Agent Plugins 1.0 (led by Vercel; supported by OpenAI/AWS/GitHub/Microsoft) | **Adapted**: `adapt` emits its `plugin.json`; this spec does **not claim conformance** |
| A2A / Agent2Agent (now hosted by AAIF) | **Adapted**: `adapt` emits `agent-card.json` |
| IETF AAT (`draft-sharif-agent-audit-trail`) | **Referenced**: `trace.json` field design is informed by it; no conformance claimed |
| IETF GAR (`draft-sato-soos-gar`) | **Referenced**: audit-package thinking informed the design; no conformance claimed |
| OWASP AISVS C9.4 | **Referenced**: boundary and provenance design informed by it; no compliance claimed |

> ⚠️ Both GAR and AAT are **individual Internet-Drafts with no formal IETF standing**; AAIF is a hosting foundation, not a protocol. This project does not borrow the authority of any standard.

---

## Roadmap

- [x] **v0.3** individual verifiable edition: source of truth + compat declaration + closure + verify + adapters
- [ ] **v0.4** `--sign`: real GPG / Ed25519 signing (replacing digest-only)
- [ ] **v0.4** real JSON Schema validation (currently structural declaration only)
- [ ] **v0.5** test-suite runner: unit/regression/sensitivity/stress are declarations today; make them execute
- [ ] **v0.5** public record page: turn every evidence id into searchable history
- [ ] **v0.6** global consistency checks for multi-agent orchestration (against "every part right, the whole wrong")
- [ ] **v1.0** spec freeze

---

## FAQ

**How is this different from a README claiming "our AI is accurate"?**
The difference is that `verify` can FAIL. Adjectives cannot.

**My scenario uses an LLM. Can I still use this?**
Yes. Either accept that the conclusion is not reproducible (and say so honestly), or record the full model closure — `attest/closure.json` already reserves the fields. There is no third option.

**Why Go rather than Python or Rust?**
Go compiles to **a single static binary** and cross-compiles for three platforms in seconds — whoever receives a package can verify it without installing any runtime. Rust can do the same, but it pulls in `serde@@ / `sha2@@ and friends, **breaking the zero-third-party-dependency rule**. Python's problem is not the language, it is distribution (no Python installed, no tool).
The **executor** can be in any language — the CLI only requires the contract: read `argv[1]`, print `{"output": {...}}` to stdout.

**What scenarios fit?**
Anything requiring **reasoning, data derivation, or result verification**: engineering estimation, quantitative forecasting, research argumentation, project appraisal, risk review. Not for copywriting, notifications or chit-chat.

**Why the name WorkAgent?**
Because it delivers **work product**, not conversation.

---

## Contributing

The most valuable contributions are **a real-world executor** or **a counterexample where the conclusion turned out not to be reproducible**. See [CONTRIBUTING.md](CONTRIBUTING.md).

Pull requests that fail `verify` will not be merged. That is not strictness — that is the entire point of this project.

---

## License & disclaimer

[MIT](LICENSE) © 2026 WorkAgent contributors.

**Disclaimer**: this project provides **tooling and a format for verifiability**, not business conclusions. The correctness of any conclusion produced with it is the responsibility of that package's author and signer. Always read a package's `x-workagent/boundary.json` and `compat.json`.
