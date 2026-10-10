# Example: bid pricing

This example demonstrates WorkAgent's minimal loop: **input → conclusion → evidence bundle**.

## Run

```bash
workagent run    --input examples/bid_quote/input.json .
workagent verify .
```

(`workagent` is the CLI you build: `go build -o workagent ./cmd/workagent`)

## Why this example counts as "verifiable"

| Condition | This example |
|---|---|
| The executor is deterministic (no randomness, no network, no time dependence) | yes |
| Input and output have JSON Schemas | yes |
| The conclusion can be verified by replaying and comparing digests | yes (`verify`) |
| Assumptions and boundaries are declared explicitly | yes (`provenance/assumptions.json`, `x-workagent/boundary.json`) |
| Out-of-range input is refused rather than force-computed | yes (`stop_condition`) |

## Turning it into your own scenario

1. Copy this directory to `examples/your_case/`.
2. Change `executor.py` (keep the contract: read `argv[1]`, write `{"output": {...}}` to
   stdout) — **any language will do**.
3. Change `inputs.schema.json` and `artifacts/output_schema.json` at the package root.
4. Change the executor command in `agent.yaml`.
5. Re-run `check / adapt / attest / run / verify / seal`.

> The CLI handles the `python` / `python3` difference for you: if the declared interpreter is
> missing, it tries the other one.
