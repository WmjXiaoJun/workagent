# Lab report flagging

**Form**: verification ｜ **Reproducibility**: L2 / L1

> ⚠️ **This example is not a medical diagnosis.** The reference intervals are example values; in
> practice use the intervals printed on the laboratory report.

## Reference intervals (example, adults)

| Item | Male | Female | Unit |
|---|---|---|---|
| ALT | 9 ~ 50 | 7 ~ 40 | U/L |
| AST | 15 ~ 40 | 13 ~ 35 | U/L |
| Fasting glucose | 3.9 ~ 6.1 | 3.9 ~ 6.1 | mmol/L |
| Creatinine | 57 ~ 97 | 41 ~ 73 | umol/L |
| White blood cells | 3.5 ~ 9.5 | 3.5 ~ 9.5 | 10^9/L |
| Haemoglobin | 130 ~ 175 | 115 ~ 150 | g/L |
| Platelets | 125 ~ 350 | 125 ~ 350 | 10^9/L |
| Total cholesterol | < 5.2 | < 5.2 | mmol/L |

**Critical values** (example): glucose < 2.8 or > 22.2; WBC < 1.5 or > 30; platelets < 30 or > 1000;
haemoglobin < 60.

## Checks

| Code | Check | Level |
|---|---|---|
| CRIT-01 | Reaches a critical value | error |
| REF-01 / REF-02 | Below the lower / above the upper limit | warn |
| LAB-02 | The value could not be parsed | warn |
| LAB-03 | The item is not in the reference table; not judged | info |

## Run

```bash
workagent run --case lab_report --input examples/lab_report/input.natural.json .
workagent verify .
```
