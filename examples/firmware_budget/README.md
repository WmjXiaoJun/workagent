# Firmware resource budget (Flash / RAM / task stacks)

**Form**: estimation ｜ **Reproducibility**: L3 (fully deterministic, no LLM)

## Checks

| Code | Check | Level |
|---|---|---|
| FR-01 | Flash usage exceeds capacity | error |
| FR-02 | Flash usage exceeds the warning threshold (85% by default) | warn |
| FR-03 | RAM usage exceeds capacity | error |
| FR-04 | RAM usage exceeds the warning threshold (80% by default) | warn |
| FR-05 | A task stack margin is below the safety floor (20% by default) | error |
| FR-06 | A stack peak exceeds its allocation (an overflow has already happened) | error |

**FR-05 is the most valuable check**: stack overflow is the hardest fault to diagnose in the field,
with wildly varied symptoms. Feed in the RTOS's measured stack high-water marks and an insufficient
margin reports an error immediately, instead of waiting for it to blow up on site.

## Output

- Total usage and headroom (Flash and RAM separately)
- The five largest modules, to locate what to optimise
- Per-task stack margin, sorted **from the smallest margin upwards**

## Run

```bash
workagent run --case firmware_budget --input examples/firmware_budget/input.json .
workagent verify .
```

In the sample data Flash usage is 910/1024 KB (about 88.9%, triggering the FR-02 warning) and the
communication task's stack margin is about 16% (below 20%, triggering the FR-05 error) — both very
common states in real projects.
