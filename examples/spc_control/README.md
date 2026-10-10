# SPC control-chart rules

**Form**: verification ｜ **Reproducibility**: L3 (fully deterministic)

## The eight detection rules (Western Electric / Nelson)

| Code | Rule | Meaning |
|---|---|---|
| R1 | 1 point beyond 3σ | clear outlier |
| R2 | 2 of 3 consecutive points beyond 2σ on the same side | shift |
| R3 | 4 of 5 consecutive points beyond 1σ on the same side | shift |
| R4 | 8 consecutive points on the same side of the centre line | mean drift |
| R5 | 6 consecutive points monotonically increasing/decreasing | trend |
| R6 | 15 consecutive points within 1σ | stratification (unusually small variation) |
| R7 | 14 consecutive points alternating up and down | periodicity |
| R8 | 8 consecutive points beyond 1σ | mixture |

R1–R4 mean "out of control"; R5–R8 mean "warning".

## Run

```bash
workagent run --case spc_control --input examples/spc_control/input.json .
workagent verify .
```

The sample series is a **clear upward trend** (12.002 → 12.033) and should trigger R5 together with
R3/R4.
