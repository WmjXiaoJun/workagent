# Knowledge-base audit-log integrity

**Form**: verification ｜ **Reproducibility**: L3 (fully deterministic, no LLM)

## Where the rules come from

The fields and event types were **taken from a real audit log** (22,253 events carrying
`old_sha256` / `new_sha256` / `line_count` / `semantic_stale` /
`source_changed`). The sample data is **synthetic** — a real log contains your document paths and
content hashes, and does not belong in a public repository.

## The six checks

| Code | Check | Level |
|---|---|---|
| AC-01 | Per `doc_id` hash chain: `old_sha256` must equal that document's previous `new_sha256` | error |
| AC-02 | When `source_changed=true` the hash must actually change | error |
| AC-03 | When `semantic_stale=true`, `semantic_status` must be marked as needing an update | warn |
| AC-04 | `line_count` drops by more than the threshold (50% by default) — suspected truncation | error |
| AC-05 | Event timestamps for one document must not go backwards | warn |
| AC-06 | A non-ingest event references a `doc_id` that was never established | warn |

### One critical implementation point

**Do not sort globally by timestamp before checking.** An audit log is an append-only ordered stream,
and sorting by timestamp silently removes the very thing you are trying to detect — timestamps going
backwards. The bug disappears without trace.

The correct approach is to **group by document and keep the log's original order within each group**.
We hit this on the first implementation.

## Measured output

```text
Audit verdict "inconsistent": 9 events / 4 documents, 3 error(s), 3 warning(s)

[error] AC-01 Hash chain broken: doc=doc_beta id=a04 old=bbbb9999... expected bbbb1111...
[error] AC-02 source_changed is true but the hash did not change: doc=doc_delta id=a08
[error] AC-04 Line count dropped abnormally: doc=doc_gamma 300 lines -> 90 lines (70% drop)
[warn]  AC-05 Timestamp went backwards: doc=doc_beta previous ts=...300 this ts=...250
[warn]  AC-03 Marked semantically stale but the status was not updated: doc=doc_gamma
[warn]  AC-06 References a document that was never established: doc=doc_missing
```

All six rules fire exactly once, with no false positives.

## Run

```bash
workagent run --case audit_chain --input examples/audit_chain/input.json .
workagent verify .
```
