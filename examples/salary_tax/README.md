# Payroll income tax

**Form**: estimation ｜ **Reproducibility**: L3 (fully deterministic, no LLM)

## Rule source

China's Individual Income Tax Law: the **cumulative withholding method** for resident comprehensive
income, with the 7-bracket progressive withholding table:

| Cumulative taxable income | Rate | Quick deduction |
|---|---|---|
| ≤ 36,000 | 3% | 0 |
| 36,000 ~ 144,000 | 10% | 2,520 |
| 144,000 ~ 300,000 | 20% | 16,920 |
| 300,000 ~ 420,000 | 25% | 31,920 |
| 420,000 ~ 660,000 | 30% | 52,920 |
| 660,000 ~ 960,000 | 35% | 85,920 |
| > 960,000 | 45% | 181,920 |

Standard deduction: 5,000 per month.

```text
cumulative taxable income = cumulative income
                          - 5000 x months
                          - cumulative special deductions
                          - cumulative additional special deductions
withholding this period   = cumulative tax - tax already withheld
```

## Run

```bash
workagent run --case salary_tax --input examples/salary_tax/input.json .
workagent verify .
```

## Why it is reproducible

No model, no network, no time dependence — the same input necessarily yields the same output digest.
That is **L3**.
