# WorkAgent package specification v0.3.0

> Status: early draft; interfaces may change. This document describes the package format and contract
> of **spec version 0.3.0**. (The `spec_version` field tracks the *format*; `version` tracks a release.
> They move independently — the format did not change in every release.)

## 1. Terminology

| Term | Meaning |
|---|---|
| **Package** | a directory containing `agent.yaml` and the required files |
| **Source of truth** | `agent.yaml`. The only metadata entry point meant to be edited by hand |
| **Adapters** | manifests generated from the source of truth, aimed at other ecosystems |
| **Executor** | the program that actually produces the conclusion; follows §10 |
| **Evidence bundle** | the input, conclusion, summary and metadata produced by one execution |
| **Output digest** | SHA-256 of the canonical JSON of the conclusion |
| **Attestation closure** | the set of digests of every component needed to reproduce the conclusion |

## 2. Package model and required files

The file list is defined by `requiredFiles` in `cmd/workagent/main.go` (single source of
truth). `check` validates it item by item.

Key conventions:

1. The package root must contain `agent.yaml`.
2. Every JSON file must be valid UTF-8 JSON.
3. `adapters/` and `out/` are generated and are **not committed**.
4. Apart from `adapters/`, no file may be both tool-generated and hand-edited.

## 3. Source of truth and adapters

- Source of truth: `agent.yaml`.
- `adapt` reads it and generates:
  - `adapters/plugin.json` — for Agent Plugins 1.0 clients
  - `adapters/agent-card.json` — for A2A discovery
- **Invariant**: no command may modify the contents of `agent.yaml` (`attest` only reads its hash).

## 4. Compatibility and degradation (`compat.json`)

`compat.json` must contain:

| Field | Required | Meaning |
|---|---|---|
| `principle` | yes | one sentence stating the compatibility strategy |
| `min_platform` | yes | the minimum platform requirement |
| `capability_matrix` | yes | per capability on a "legacy platform": `full` / `degraded` / `lost` |
| `degradation` | yes | what is preserved, what is lost, and an **explicit statement** |

**Normative**: any capability marked `lost` in `capability_matrix` must **not** be described as
still working in `degradation.statement`.

## 5. Provenance

### `provenance/trace.json`
Every `run` appends a record containing at least: `input_digest`, `output_digest`,
`evidence_id`, `executor`, `at`. `verify` replays from it.

### `provenance/data_sources.json`
Every data source that feeds a conclusion must record its origin, snapshot version, retrieval time,
hash and access rights. **An unregistered data source must not participate in computation.**

### `provenance/assumptions.json`
Every assumption must carry `statement`, `basis`, `impact` and `if_false`.
A separate `excluded` list records **excluded scenarios**.

**Normative**: any implicit assumption that is not registered counts as a defect in the package.

## 6. Attestation closure

The `closure` in `attest/closure.json` must cover:

- digests of the executor, input schema, workflow and manifest
- the runtime environment: build toolchain versions, platform, numeric environment
  (`PYTHONHASHSEED` / `LC_ALL` / `TZ`)
- the executor command (`executor_command`); **the executor's own runtime version** (Python / Node)
  is the executor author's responsibility to record

**If an LLM step is enabled**, these must also be filled in:
`model_weights_digest`, `tokenizer_digest`, `chat_template_digest`,
`engine_build_digest`, `decoding_config`.

**Normative**: while any of those LLM fields is empty and `llm_used` is true, the package must
**not** claim its conclusion is reproducible.

## 7. Boundary

`x-workagent/boundary.json` must contain `valid_when`, `invalid_when`,
`confidence_rating` and `stop_condition`.

**Normative**: on hitting `invalid_when`, the executor must **exit with a non-zero status** and
must not write a conclusion to stdout.

## 8. Test suites

Four kinds under `x-workagent/test_suite/`:

| File | Purpose |
|---|---|
| `unit.json` | single-node logic checks |
| `regression.json` | fixed historical baselines; detects iteration drift |
| `sensitivity.json` | conclusion stability under parameter variation |
| `stress.json` | extreme and boundary inputs |

> ⚠️ **Current state**: in v0.3.0 these four are **declarations only** — the toolchain does not execute
> them yet. They are on the roadmap.

## 9. Signature

`signature/agent.sig` is produced by `seal` and contains `scheme`,
`package_digest` and a per-file hash list.

**Normative**: `scheme` must be stated truthfully. Without a private key it must be
`digest-only`; words such as `signed` must **not** be used.

## 10. Executor contract

```text
input:  argv[1] = path to the input JSON
success: exit code 0, stdout = {"output": {...}} (valid JSON)
failure: non-zero exit code, reason on stderr
```

Constraints:

- Must be **deterministic** (no randomness, no network, no time dependence), or register the
  non-reproducible part in the closure.
- Must not write anything to stdout other than the result JSON.

## 11. Versioning and change policy

- Semantic versioning.
- **Add, never change**: compatibility comes from added directories and fields, not from changing the
  meaning of existing ones.
- A breaking change must raise the major version and describe the migration path in `CHANGELOG.md`.

## 12. LLM steps and reproducibility levels

### 12.1 Declaration

`execution.llm_steps` in `agent.yaml` declares model steps:

```yaml
llm_steps:
  - id: "extract"
    provider: "ollama"              # see: workagent models
    model: "qwen2.5:7b"
    prompt: "path/to/prompt.md"     # supports the {{INPUT_JSON}} placeholder
    temperature: 0
    seed: 42
    json_mode: true
    when_input_has: "description"   # skip when the input lacks this field (structured input = zero network)
```

### 12.2 Contract

1. Model steps run **before** the deterministic executor.
2. The model output is parsed into a structured intermediate representation and written to
   `out/run/input.enriched.json` together with the original input.
3. What the executor reads **must be structured data** — that is the premise of a reproducible verdict.
4. The raw model response is written to `provenance/llm/<id>.json` with `prompt_digest` and
   `response_digest`.

### 12.3 Replay

`run --replay` and `verify` use the recorded response and **never touch the network**.

**Normative**: `prompt_digest` must be checked before replay; on mismatch the tool must **fail**,
never silently use a stale recording.

### 12.4 Reproducibility levels

| Level | Meaning | Condition |
|---|---|---|
| L3 | fully reproducible | no LLM step |
| L2 | decision reproducible | local open-weights provider + `temperature=0` + fixed `seed` |
| L1 | record reproducible | everything else; the decision process replays, the model does not |
| L0 | not reproducible | `temperature>0` |

**Normative**: the level must be written truthfully into the `reproducibility` field of
`evidence.json`. If `verify --live` measures a response that differs from the recording, it
must report FAIL and must **not** downgrade it to a warning.

### 12.5 Recordings must not be destroyed by verification

**Normative**: `verify --live` and every other verification path must **not overwrite** existing
recordings under `provenance/llm/`.

## 13. Presentation contract (`messages.json`)

Executors emit **facts** (`code` + `args`, and `conclusion_args`); the prose lives in
`messages.json` and is rendered by the toolchain according to the output language. See
`docs/MESSAGES.md`.

**Normative**: rendering must not participate in the output digest. `output_digest` is computed
from the executor's raw structured output, before any rendering — otherwise the same facts would
digest differently in different languages and `verify` would fail.

**Normative**: one `code` may carry exactly one message. Two different messages under the same
code are silently rendered as one of them.
