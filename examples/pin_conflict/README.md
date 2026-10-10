# Pin / IRQ / DMA resource conflicts

**Form**: verification ｜ **Reproducibility**: L3 (fully deterministic)

## Checks

| Code | Check | Level |
|---|---|---|
| PIN-01 | One pin assigned to two different signals | error |
| PIN-02 | A reserved debug pin is in use | error |
| PIN-03 | A pin name does not match `P<port><number>` | error |
| IRQ-01 | Duplicate preemption priority (this rule set requires uniqueness) | error |
| IRQ-02 | An interrupt priority is outside the MCU's allowed range | error |
| DMA-01 | A DMA channel is declared twice | warn |
| DMA-02 | A DMA channel is assigned to multiple peripherals | error |

## Why this scenario is worth building

**A conflict is a bug, it is verifiable at compile time, and the rules are fully deterministic.**
Doing this by hand misses things after a couple of projects; as a deterministic tool it runs on every
hardware change at zero cost.

## Run

```bash
workagent run --case pin_conflict --input examples/pin_conflict/input.json .
workagent verify .
```

The sample data contains four mistakes that really do happen: PA9 claimed by both USART1_TX and
TIM1_CH2, PA13 used as an LED although it is a debug pin, priority 5 shared by two interrupts,
priority 20 outside the 0~15 range, plus a pin named `P13`.
