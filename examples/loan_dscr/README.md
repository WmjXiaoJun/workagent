# Loan debt-service coverage

**Form**: estimation ｜ **Reproducibility**: L3

## Rules

```text
DSCR = (net profit + depreciation + amortisation + interest) / (principal amortisation + interest)
ICR  = EBIT / interest expense
```

| DSCR | Verdict |
|---|---|
| ≥ 1.25 | passed |
| 1.10 ~ 1.25 | needs review |
| < 1.10 | rejected |

The thresholds can be overridden in the input (`dscr_min` / `dscr_review`) and are
**deliberately not hard-coded** — policies differ between lenders.

## Run

```bash
workagent run --case loan_dscr --input examples/loan_dscr/input.json .
workagent verify .
```
