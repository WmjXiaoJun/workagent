# Message catalog (`messages.json`)

**Executors do not write prose. The prose lives here.**

[中文](MESSAGES.zh-CN.md) · **English**

## Why it is split this way

An executor is a **separate process**: it cannot see the CLI's language setting. More importantly,
wording is **content**, and its source of truth belongs in the delivery package alongside
`agent.yaml` — not scattered across twelve Python files.

```text
executor (Python)   →  emits code + args only, no natural language at all
messages.json       →  the wording, zh and en side by side, so a missing one is visible
workagent (Go)      →  renders according to --lang
```

## One iron rule: rendering never takes part in hashing

`output_digest` is taken from the executor's **raw structured output**; rendering happens
**after** the digest is computed.

If rendering fed the digest, a Chinese run and an English run would produce two different digests and
`verify` would fail immediately. Measured:

```text
$ workagent run --case audit_chain .
  evidence bundle: ed6532b74aec41d1
$ workagent run --case audit_chain . --lang en
  Evidence bundle: ed6532b74aec41d1      ← the same
```

The evidence bundle's `output.json` keeps the raw output; what a human reads is `summary.md`
and `records/`.

## Format

```json
{
  "messages": {
    "audit_chain.CONCLUSION": {
      "zh": "核验判定「{verdict}」：事件 {events} 条 / 文档 {docs} 个",
      "en": "Audit verdict \"{verdict}\": {events} events / {docs} documents"
    },
    "audit_chain.AC-01": { "zh": "...", "en": "..." }
  },
  "verdicts": {
    "audit_chain": { "不一致": { "zh": "不一致", "en": "inconsistent" } }
  }
}
```

- Keys are `<caseID>.<CODE>`; `CONCLUSION` is reserved for the conclusion sentence.
- `{name}` placeholders are filled from `args`. **A missing key is left as-is**, so an
  unfilled slot is visible rather than silently empty.
- `{verdict}` inside a conclusion is translated through `verdicts` first, so an English
  sentence never contains a Chinese verdict word.
- **If English is missing, nothing is rendered** and the executor's own text is used instead — we do
  not pretend something is translated. `check` fails on such an entry.
- `conclusion_code` lets an executor pick a *different message* based on the facts (for example
  `spc_control`'s two conclusion shapes). The facts choose the code; the catalog still supplies the words.

## What an executor does

Add `args` to a finding and `conclusion_args` to the output:

```python
findings.append({
    "level": "error", "code": "AC-01",
    "args": {"doc": did, "id": eid, "got": old[:12], "want": last_hash[did][:12]},
    "message": "哈希链断裂：" + did,        # compatibility fallback: older consumers read only message
})

out["conclusion_args"] = {"verdict": verdict, "events": n, "docs": m, ...}
```

`message` is **kept**: the field already exists in the specification, and removing it would be a
breaking change. When `args` is present the renderer prefers `args` + the catalog; without it,
`message` is shown verbatim.

## One code, one message

The catalog maps `code → template`, so **a single code must not carry two different sentences**.

`audit_chain` originally recorded both "hash chain broken" and "first ingest carrying an
old_sha256" under `AC-01` — the second was silently rendered with the first's wording, **wrongly and
without any signal**. It is now split into `AC-01` and `AC-01F`.

Six of the twelve scenarios had such a collision during migration. That is why this is written down as
a rule rather than left to memory.
