# Expense compliance review

**Form**: verification ｜ **Reproducibility**: L2 / L1

The model only reads the claim into structured line items; **every compliance judgement is made by
deterministic code**.

## Checks

| Code | Check | Level |
|---|---|---|
| EXP-01 | Claimed total does not match the line-item total | error |
| EXP-02 | A single item exceeds the policy cap | error |
| EXP-03 | A category cap is exceeded (meals 1000 / office 500 / lodging 600) | error |
| EXP-04 | The expense date is after the submission date, or past the claim window | error |
| EXP-05 | The category is not on the allow-list | warn |
| EXP-06 | No invoice number | warn |
| EXP-07 | The model flagged uncertain items | warn |

## Run

```bash
# first time: call the model for real and record the response
workagent run --case expense_audit --input examples/expense_audit/input.natural.json .
# afterwards (including CI): replay offline, no key needed
workagent run --case expense_audit --input examples/expense_audit/input.natural.json . --replay
workagent verify .
```

The sample data deliberately contains three classes of problem: a claimed total of 3120 against
line items of 3260, a meal expense of 1180 over the category cap, and several items with no invoice
number.
