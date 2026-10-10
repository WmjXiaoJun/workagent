# How to divide the industry: a slicing method for solo developers

> This answers one concrete question: **I do not know which industry to build for. How do I choose?**

[中文](VERTICALS.zh-CN.md) · **English**

## 0. First, drop one assumption

**Do not divide by industry** (finance / engineering / healthcare / legal). Industry is an outcome,
not a selection criterion.

Three things actually decide whether something is worth building:

1. **Can the conclusion be verified after the fact?** — this is the only moat, and the only thing
   that accumulates
2. **Can the judgement be written down as rules?** — whatever can be, give to code; the model only
   handles the edges
3. **Can you get the input?** — structured data, or at least semi-structured text

By those three, the industry re-slices into **five delivery forms**.

---

## 1. The five delivery forms

| # | Form | Input → output | The model's role | How it is verified | Repro level | Fit for a solo dev |
|---|---|---|---|---|---|---|
| ① | **Estimation** | parameters → numbers | none, or extraction only | compare against later actuals | **L3 / L2** | ★★★★★ |
| ② | **Verification** | a pile of material → pass / return + reasons | extraction + classification | human review as the baseline | **L2 / L1** | ★★★★★ |
| ③ | **Extraction** | unstructured text → structured data | **the lead** | field-by-field against the original | L1 | ★★★★ |
| ④ | **Scheduling** | constraints → a plan | supporting | constraint satisfaction is checkable | L3 | ★★★ |
| ⑤ | **Projection** | conditions → conclusions / advice | **the lead** | weak; only known afterwards | L1 | ★★ |

### The governing rule

> **The earlier the model sits in the pipeline (understanding, extracting), the less reproducible the
> result; the later it sits (polishing the wording), the easier the whole thing is.**

So the best landing spots are **① and ②** — they naturally grow into this shape:

```text
unstructured input ──[model extracts]──▶ structured IR ──[deterministic code]──▶ conclusion + evidence
                          ↑ not reproducible, record and replay    ↑ reproducible, L3
```

**"The model only translates; code decides."** That one rule settles 80% of reproducibility problems.

---

## 2. Choosing a quadrant: verifiability × model dependence

```text
                model dependence  low  ←──────────────→  high
verifiability high  ┌──────────────────┬──────────────────┐
                    │  ① Estimation    │  ③ Extraction    │
                    │  ② Verification  │                  │
     ↑              │  ★ best landing  │  doable, but you │
                    │                    │  must record    │
                    ├──────────────────┼──────────────────┤
                    │  ④ Scheduling    │  ⑤ Projection    │
     low            │  narrow value    │  ✗ do not touch  │
                    └──────────────────┴──────────────────┘
```

**The top-right corner (highly verifiable + highly model-dependent) is a false premise**: deep model
involvement *and* a verifiable conclusion is not technically possible today. ⑤ is "do not touch" not
because it makes no money, but because **what you build cannot be shown to be right** — and "can be
shown" is precisely why this project exists.

---

## 3. Concrete examples per form

### ① Estimation
- Bill-of-quantities pricing (this repository's example)
- Equipment selection and whole-life cost
- Project tax burden / cash flow
- Performance bonus and commission
- Freight and route cost

### ② Verification
- **Expense report review** (clear rules, high volume, everyone has felt the pain)
- Contract terms against internal standards
- Bid-document compliance / disqualifying clauses
- Purchase three-way match (order – receipt – invoice)
- Financial statement articulation checks

### ③ Extraction
- Contract key terms (amount, duration, penalty clauses, payment milestones)
- Invoice / receipt fields
- Résumé structuring
- Medical record / lab report fields
- Tender qualification conditions

### ④ Scheduling
- Construction schedule and resource levelling
- Production line scheduling
- Inspection / delivery routes
- Shift rosters

### ⑤ Projection
- Feasibility studies
- Technology selection advice
- Risk registers
- Due-diligence conclusions

---

## 4. Picking the first one: four sieves

**All four must pass.** Fail one and change direction:

| # | Sieve | The signal that it fails |
|---|---|---|
| 1 | **Can you get real outcomes to compare against?** | "accuracy can only be judged by feel" → change |
| 2 | **Can the rules be written down?** | "it is all experience, I cannot explain it" → change (that is a pure model, unreproducible) |
| 3 | **Can you get the input?** | "the data lives in the client's system, I cannot reach it" → change |
| 4 | **Is one delivery worth paying for, or worth the time saved?** | "saves 5 minutes" → change (nobody changes habits for that) |

> One more **trust signal**: real validation is not "the user says it is good", it is
> **whether the user is willing to hand you real data to process**. When they drop a dozen real
> documents in a row, that is a signal. Trying it once for fun is not.

---

## 5. Where this repository's examples sit

`examples/bid_quote/` demonstrates the skeleton of both form ① and form ②:

| Input file | Path | Repro level |
|---|---|---|
| `input.json` | structured parameters → deterministic computation | **L3, fully reproducible**, no network |
| `input.natural.json` | natural language → model extraction → deterministic computation | **L2** (local open weights + temp 0) or **L1** (cloud model, replayed from a recording) |

Both paths share one executor: **the model only turns "120 tonnes of steel at 4850 a tonne" into
`{quantity: 120, unit_cost: 4850}`; how the money is computed is code's business.**

The repository now ships **12 scenarios across both forms** (5 estimation, 7 verification) — see
`docs/EXAMPLES.md`. To build your own, change only three things: `executor.py` (the rules),
the extraction prompt (the extraction target), and `boundary.json` (the failure boundary).
