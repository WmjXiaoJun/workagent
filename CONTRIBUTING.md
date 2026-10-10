# Contributing

## What we need most

| Priority | We want | Why |
|---|---|---|
| P0 | **Executors for real scenarios** | One concrete, verifiable, minimal scenario is worth more than ten generic demos |
| P0 | **Counter-examples**: cases whose conclusions are not reproducible | They help us state the reproducibility bar precisely |
| P1 | Compatibility reports | Did your package run on some legacy platform? Which capabilities were lost? |
| P1 | Adapters for standards | Agent Plugins / A2A / others |
| P2 | Documentation translation | Improvements to README.zh-CN.md |

## What we do **not** accept

- General-purpose AI assistants / chatbots
- Platforms, frameworks, orchestration engines
- Core tooling that pulls in third-party dependencies (`go.mod` must stay free of `require`)
- Scenarios that cannot be verified (domains where "right" is undecidable)

## Before you submit

```bash
go build -o workagent ./cmd/workagent
./workagent check .
./workagent run --all . --replay
./workagent verify --all .      # must PASS
```

**`verify` failing = not reproducible = not mergeable.**

## Design constraints (read before changing anything)

1. **One source of truth**: `agent.yaml`. `adapters/` is always generated and never hand-edited.
2. **Zero third-party dependencies**: `cmd/workagent/` uses only the Go standard library; `go.mod`
   must not contain a `require`.
3. **Add, never change**: compatibility comes from added directories, not from editing existing fields.
4. **Honesty**: without a private key you may not write "signed"; when unsure, write "unsure".

## Commit messages

```text
<type>: <one-line summary>
```

`type` is one of `spec` / `tool` / `example` / `docs` / `compat` / `fix`.

## Licence

By contributing you agree to release your contribution under the MIT licence.
