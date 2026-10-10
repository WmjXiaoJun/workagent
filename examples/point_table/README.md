# Modbus point-table audit

**Form**: verification ｜ **Reproducibility**: L2 / L1

**Every project needs this, and getting it wrong is a field incident.** The rules are fully
deterministic; the model only reads the point table into structured data.

## Checks

| Code | Check | Level |
|---|---|---|
| PT-01 | Address ranges overlap within the same function code | error |
| PT-02 | Unknown data type, so the register count cannot be derived | warn |
| PT-03 | The range's lower bound is above its upper bound | error |
| PT-04 | The address is outside the Modbus addressable range | error |
| PT-05 | Missing engineering unit | warn |
| PT-06 | Illegal function code | error |

## Modbus reference-address normalisation

Field point tables routinely use reference numbers such as `40001`, where the **actual offset is
minus 10000**. Skipping the normalisation causes a flood of false findings:

```text
4xxxxx → holding registers,  offset = addr - 40001
3xxxxx → input registers,    offset = addr - 30001
1xxxxx → discrete inputs,    offset = addr - 10001
0xxxxx → coils,              offset = addr - 1
```

## Run

```bash
workagent run --case point_table --input examples/point_table/input.natural.json .
workagent verify .
```

The sample point table hides four typical mistakes: two points both claiming 40001 (overlap), a
turbidity range written as 100~0 (inverted), a level address of 70000 (out of range), and a
temperature with no unit.
