# Public record index

> Every conclusion and its evidence is archived here. **This is the only asset that compounds over time.**

Records are **append-only**: each `workagent record` allocates a new number, and `INDEX.md` is rebuilt from `records/*.md`; hand edits are lost.

Raw inputs **never enter the repository** (only their sha256), which is why these records can be public.

**13** records ｜ package version `0.11.0`

| # | Date | Scenario | Form | Verdict | Repro | Output digest |
|---|---|---|---|---|---|---|
| [0013](0013-spc_control-20261010.md) | 2026-10-10 | `spc_control` | Verification | **失控** | L3 | `67a870863eac0dcb` |
| [0012](0012-audit_chain-20261010.md) | 2026-10-10 | `audit_chain` | Verification | **不一致** | L3 | `ea70c91625e07e08` |
| [0011](0011-points_reconcile-20261010.md) | 2026-10-10 | `points_reconcile` | Verification | **不平** | L3 | `0aa873ef1841e21d` |
| [0010](0010-firmware_budget-20261010.md) | 2026-10-10 | `firmware_budget` | Estimation | **超限** | L3 | `959acbbc1f9b2a22` |
| [0009](0009-modbus_poll-20261010.md) | 2026-10-10 | `modbus_poll` | Estimation | **满足** | L3 | `b14c43001bd4dc77` |
| [0008](0008-loan_dscr-20261010.md) | 2026-10-10 | `loan_dscr` | Estimation | **通过** | L3 | `166f7706ca5e2c22` |
| [0007](0007-salary_tax-20261010.md) | 2026-10-10 | `salary_tax` | Estimation | **已测算** | L3 | `39e92b1797e8dcbd` |
| [0006](0006-pin_conflict-20261010.md) | 2026-10-10 | `pin_conflict` | Verification | **存在冲突** | L3 | `566f5ca7445cca7f` |
| [0005](0005-spc_control-20261010.md) | 2026-10-10 | `spc_control` | Verification | **失控** | L3 | `67a870863eac0dcb` |
| [0004](0004-point_table-20261010.md) | 2026-10-10 | `point_table` | Verification | **不通过** | L2/L1 | `be320b395bb68368` |
| [0003](0003-lab_report-20261010.md) | 2026-10-10 | `lab_report` | Verification | **异常** | L2/L1 | `6d0f45ef3edc7a43` |
| [0002](0002-expense_audit-20261010.md) | 2026-10-10 | `expense_audit` | Verification | **退回** | L2/L1 | `f746d8e68f87fb04` |
| [0001](0001-bid_quote-20261010.md) | 2026-10-10 | `bid_quote` | Estimation | **已测算** | L2/L1 | `2775a35462d805b5` |
