# Compatibility guide

> The binding constraint: **new capabilities ship, old platforms must not break.**

[中文](COMPAT.zh-CN.md) · **English**

## 1. Strategy: add directories, never change fields

```text
agent.yaml (source of truth, field structure unchanged)
   │
   ├── APS base fields ─────▶ readable by any legacy platform
   └── x-workagent extension ─▶ legacy platforms ignore it; new platforms parse it
```

Legacy platforms **ignore unknown top-level keys**, so the extension block cannot break parsing.
That is the premise this scheme rests on.

> ⚠️ If your legacy platform **errors on** unknown fields instead of ignoring them, it does not meet
> the `min_platform` requirement. Say so in `compat.json`, or move the extension block into a
> separate file and reference it.

## 2. What is lost on a legacy platform

For this repository, taken from `compat.json`'s `capability_matrix`:

| Capability | Depends on | Legacy platform |
|---|---|---|
| Reading the manifest, input/output validation, workflow execution, `summary.md` | APS | kept |
| Boundary enforcement | `x-workagent` | **lost** |
| Regression baseline comparison | `x-workagent` | **lost** |
| Conclusion reproducibility check | `x-workagent` + `attest` | **lost** |
| Package integrity verification | `x-workagent` + `signature` | **lost** |

## 3. How to declare the degradation

`compat.json`'s `degradation.legacy_platform` must state three things:

1. `preserved` — what still works;
2. `lost` — what is gone;
3. `statement` — **one sentence, addressed to the end user**, saying what that means.

A reference wording:

> On a platform that supports only the APS base fields, this package still runs and produces a
> conclusion; but that conclusion cannot be checked for reproducibility, cannot be verified for
> integrity, and is not protected by the failure-boundary rules. Do not base high-risk decisions on it.

**This sentence is the most underrated part of the project.** Most projects leave the user to guess.
We write it into a machine-readable file.

## 4. Migrating from v0.2 to v0.3

| v0.2 | v0.3 | Action |
|---|---|---|
| `agent.yaml` | `agent.yaml` | kept, **field structure unchanged** |
| — | `compat.json` | added |
| — | `attest/closure.json` | added |
| `artifacts/summary.md` | `summary.md` + `summary.template.md` | template and artifact split |
| `provenance/trace.json` | same | gains `input_digest` / `output_digest` / `evidence_id` |
| `signature/agent.sig` | same | explicit `scheme` field; vague wording forbidden |
| — | `adapters/` | added (generated) |

**The migration does not break legacy platforms**: every change is an added file, and no existing
field changed meaning.

## 5. Adapting to other ecosystems

`workagent adapt` generates:

| Artifact | Target | What it is |
|---|---|---|
| `adapters/plugin.json` | Agent Plugins 1.0 | a portable plugin manifest |
| `adapters/agent-card.json` | A2A | a capability self-description, for discovery |

**Note**: these are **adaptations**, not **conformances**. This project does not claim to satisfy
*every* requirement of those specifications — Agent Plugins, for example, requires its manifest to be
`plugin.json`, while our source of truth is `agent.yaml` and the adapter layer only maps to it.
Say "**can be exported as …**", never "**conforms to …**".

## 6. Anti-patterns

| Anti-pattern | Why it is wrong | Do this instead |
|---|---|---|
| Editing `adapters/` by hand | The next generation overwrites it, and the source of truth drifts from the adapter | Edit `agent.yaml` and re-run `adapt` |
| Renaming `agent.yaml` fields to match a new standard | Breaks legacy compatibility | Map them in the adapter layer |
| `compat.json` says only "fully compatible" | The user cannot know what they lose | List `lost` explicitly |
| Claiming "signed" with no private key | A false claim, worse than not signing | `scheme: digest-only` |
| Using an LLM without registering the model digests | The conclusion is unreproducible while being claimed reproducible | Fill in the closure, or label it honestly as unreproducible |
