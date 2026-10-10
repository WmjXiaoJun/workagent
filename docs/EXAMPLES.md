# Scenario list, organised by delivery form

> **Not by industry.** Industry is an outcome, not a selection criterion — for why, see [VERTICALS.md](VERTICALS.md).

```bash
workagent cases .                            # list every scenario, grouped by form
workagent run --case <ID> --input <file> .   # run one
workagent verify --all .                     # reproducibility check across all
```

## The five delivery forms

| Form | Input → output | The model's role | How it is verified | Shipped here |
|---|---|---|---|---|
| **Estimation** | parameters → numbers | none, or extraction only | compare against later actuals | ✅ 5 |
| **Verification** | a pile of material → pass / return + reasons | extraction + classification | human review as the baseline | ✅ 7 |
| **Extraction** | unstructured text → structured data | **the lead** | field-by-field against the original | ⬜ todo |
| **Scheduling** | constraints → a plan | supporting | constraint satisfaction is checkable | ⬜ todo |
| **Projection** | conditions → conclusions / advice | **the lead** | weak; only known afterwards | ⬜ todo |

```text
$ workagent cases . --lang en
WorkAgent cases — grouped by delivery form

[Estimation] 5
  bid_quote        L2/L1    Bid pricing
  salary_tax       L3       Payroll income tax
  loan_dscr        L3       Loan debt-service coverage
  modbus_poll      L3       Modbus polling load and cycle time
  firmware_budget  L3       Firmware Flash / RAM / stack budget

[Verification] 7
  expense_audit    L2/L1    Expense compliance review
  lab_report       L2/L1    Lab report flagging
  point_table      L2/L1    Modbus point-table audit
  spc_control      L3       SPC control-chart rules
  pin_conflict     L3       Pin / IRQ / DMA resource conflicts
  points_reconcile L3       Points ledger reconciliation
  audit_chain      L3       Knowledge-base audit-log integrity
```

---

## Estimation

Parameters in, numbers out. **The model, when used at all, only reads natural language into parameters.**

### `salary_tax` Payroll income tax
Computes the monthly liability with the cumulative withholding method, and reports the monthly
breakdown, the annual total and the effective rate.
**Rule source**: the 7-bracket progressive withholding table for resident comprehensive income under
China's Individual Income Tax Law (3%–45%, quick deductions 0–181,920), with a standard deduction of
5,000 per month.

### `loan_dscr` Loan debt-service coverage
`DSCR = (net profit + depreciation + amortisation + interest) / (principal amortisation + interest)`,
plus an interest-coverage ratio.
The 1.25 / 1.10 thresholds **live in the input, not in the code** — policies differ between lenders,
and hard-coding them is simply wrong.

### `bid_quote` Bid pricing
Either compute straight from structured line items (L3), or extract them from a natural-language
description first (L2). Both paths share one executor.

### `modbus_poll` Modbus polling load and cycle time
Given link parameters (baud rate / parity / stop bits) and a point table, computes **how long one
polling cycle takes**, whether it meets the target, and where the bottleneck is.
**The rules come from the Modbus frame format**: an RTU frame is address(1) + function(1) + data(2n) +
CRC(2); a character is 1 start bit + data bits + parity + stop bits; the inter-frame silence is 3.5
character times; one request carries at most 125 registers.
**The key optimisation**: adjacent registers merge into a single request — in the example, 12 points
merge down to 5 requests and one 75 ms cycle.

### `firmware_budget` Firmware Flash / RAM / stack budget
Aggregates per-module Flash and RAM usage and measured task stack high-water marks, and compares them
against the MCU capacity.
**FR-05 (stack margin below the safety floor) is the most valuable check** — stack overflow is the
hardest fault to diagnose in the field, and its symptoms are wildly varied.

---

## Verification

A pile of material goes in; a verdict and its reasons come out. **The model only extracts; every
judgement is made by deterministic code.**

### `spc_control` SPC control-chart rules
Applies eight detection rules to a series of measurements and names **which point violated which rule**.
R1–R4 mean "out of control"; R5–R8 mean "warning". The centre line and sigma can be estimated
automatically with the I-MR method (MRbar / 1.128).
**Rule source**: the Western Electric / Nelson detection rules (public standard).

### `pin_conflict` Pin / IRQ / DMA resource conflicts
Pin reuse, reserved debug pins, duplicate priorities, out-of-range priorities, DMA channel contention,
pin naming. **A conflict is a bug, it is verifiable at compile time, and the rules are fully deterministic.**

### `expense_audit` Expense compliance review
Per-item limits, per-category limits, time limits, invoice numbers, total consistency. The sample data
hides three classes of problem, and all three are caught.

### `lab_report` Lab report flagging
Sex-specific reference-interval judgement, with critical values flagged separately.
> ⚠️ **The reference intervals are example values and this conclusion is not a medical diagnosis.**
> In practice, use the intervals printed on the laboratory report.

### `point_table` Modbus point-table audit
Overlapping address ranges, data types versus register counts, inverted ranges, out-of-range
addresses, missing units, illegal function codes.
**Includes Modbus reference-address normalisation** (`40001` → offset 0) — field point tables
routinely use reference numbers, and skipping the normalisation produces a flood of false findings.

### `points_reconcile` Points ledger reconciliation
Seven invariants: the ledger chain accumulates correctly, the account balance agrees with the last
ledger entry, `available + frozen = lifetime_granted - lifetime_spent`, idempotency keys are
unique, paid orders have ledger entries, reservations are settled, and amount signs match their
entry semantics.
**The rules were validated against a real production database (zero violations) before being frozen
in.** The sample rows are synthetic; their shape is copied from the real one.
The trap is reservation semantics: on a reserve, `available` falls and `frozen` rises by
the same amount, so if `lifetime_spent` also counts the reservation, the same amount is counted twice.

### `audit_chain` Knowledge-base audit-log integrity
Six checks: hash chain continuity, a claimed source change really changing the hash, semantic-stale
status consistency, sudden drops in line count, monotonic timestamps, orphan references.
**The implementation trap**: do not sort globally by timestamp — that silently removes the very thing
you are trying to detect, namely timestamps going backwards. Group by document and keep the log's
original order within each group.

---

## The three forms not yet covered

| Form | Why not yet | What it would take |
|---|---|---|
| **Extraction** | A pure extraction "conclusion" is the structured data itself, and verifying it means field-by-field comparison against the source — one layer of judgement less than verification | an annotated source-to-field corpus |
| **Scheduling** | Judging a schedule needs multi-objective evaluation (duration / cost / resource levelling) and the thresholds resist standardisation | a verifiable scheduling constraint set |
| **Projection** | **Its conclusions cannot be verified after the fact** — which is exactly why this project does not prioritise it | not recommended; if you must, it can only be L1 and clearly labelled |

---

## Adding a scenario

1. Create `examples/<id>/` with `executor.py`, `input.json`, `boundary.json` and `README.md`.
2. If it needs model extraction, add a prompt and a natural-language input.
3. Register one entry under `execution.cases` in `agent.yaml`, setting `shape` (one of the five forms).
4. `workagent check .` validates the files; `run` establishes a baseline; `verify --all` confirms reproducibility.

## Executor contract

```text
input:   argv[1] = path to the input JSON (possibly enriched: {"input": ..., "llm": {...}})
success: exit code 0, stdout = {"output": {...}} (valid JSON)
failure: non-zero exit code, stderr gives the BOUNDARY_VIOLATION reason
```

The output should carry `subject`, `verdict`, `conclusion`, `conclusion_args`,
`findings`, `metrics`, `confidence` and `risks`.

**Executors emit facts, not prose.** Each finding carries `code` and `args`; the wording lives in
`messages.json` and is rendered by the toolchain in the output language. See `docs/MESSAGES.md`.
