# Points ledger reconciliation

**Form**: verification ｜ **Reproducibility**: L3 (fully deterministic, no LLM)

## Where the rules come from

**They were not invented; they were validated against a real production database.**

The seven invariants below were first run against a live billing database (21 ledger entries / 20
accounts / 8 orders): **zero violations**. Only then were they frozen into a reproducible scenario.

> The sample data is **synthetic** — its shape is copied from the real database, its rows are not.
> A real ledger contains users and amounts, and does not belong in a public repository.

## The seven invariants

| Code | Rule | Level |
|---|---|---|
| PR-01 | Chain accumulation: `balance_after[i] == balance_after[i-1] + signed_amount` | error |
| PR-02 | Account `available_points` == that user's last ledger `balance_after` | error |
| PR-03 | `available + frozen == lifetime_granted - lifetime_spent` | error |
| PR-04 | `idempotency_key` is globally unique | error |
| PR-05 | Every paid order has a corresponding ledger entry | error |
| PR-06 | Every reservation is settled by a capture or a refund | warn |
| PR-07 | The amount's sign matches its `entry_type` semantics | warn |

### The reserve / capture / refund trap

A real ledger's `entry_type` is not just grant/spend; it carries a full
reserve–settle–refund flow:

```text
increase the balance: grant, admin_grant, capture, refund, sub2api_capture, sub2api_refund
decrease the balance: spend, reserve, sub2api_reserve
```

**PR-03 is the easiest to get wrong under these semantics.** A reserve lowers `available` and raises
`frozen` by the same amount, so `available + frozen` should stay constant; if
`lifetime_spent` also counts the reservation, the same amount is counted twice and the identity
fails. Account u_0003 in the sample data is exactly this mistake.

## Measured output

```text
Reconciliation verdict "out of balance": 7 ledger entries / 3 accounts, 2 error(s), 1 warning(s)

[error] PR-03 Account identity does not hold: user=u_0003 available+frozen=525.00 ...
[error] PR-05 Paid order has no ledger entry: order=ap_A0005 user=u_0003 points=500
[warn]  PR-06 Reservation not settled: user=u_0003 reference=t_001 open count=1
```

## Run

```bash
workagent run --case points_reconcile --input examples/points_reconcile/input.json .
workagent verify .
```

## Boundary

**It checks self-consistency only, not business semantics.** It will not tell you whether a 500-point
grant *should* have been made; it tells you that a reservation was never settled and that a paid order
never reached the ledger. **The second kind is money.**
