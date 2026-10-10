# Modbus polling load and cycle time (RTU / TCP)

**Form**: estimation ｜ **Reproducibility**: L3 (fully deterministic, no LLM)

## Why this has to be computed

The most common field failure in host-software projects: **a point table with 300 points and a 200 ms
polling target that stalls the moment it goes live.** Computing it takes seconds; skipping it means
rework on site.

## The rules (from the Modbus frame format)

```text
RTU frame            = slave address(1) + function(1) + data(N) + CRC(2)
FC03/04 request      = 8 bytes
FC03/04 response     = 5 + 2 x register count bytes
bits per character   = 1 start bit + data bits + parity + stop bits   (8E1 = 11 bits)
character time       = bits per character / baud rate
inter-frame silence  = 3.5 character times (required by the RTU spec)
max per request      = 125 registers
```

## The key optimisation: merging adjacent registers

**Points in adjacent registers can merge into a single request.** The more contiguous the addresses,
the bigger the win. The computation reports **merged versus unmerged** side by side, so you can see
exactly what merging saves.

## Output

| Field | Meaning |
|---|---|
| `cycle_merged_ms` | one cycle after merging — **the number that decides pass or fail** |
| `cycle_solo_ms` | one cycle without merging (for comparison) |
| `requests_merged` | number of requests after merging |
| `margin_ms` | headroom against the target cycle |
| `requests` | function code, start address, register count and cost of each request |

## Run

```bash
workagent run --case modbus_poll --input examples/modbus_poll/input.json .
workagent verify .
```

The sample input is RS-485 / 9600 / 8E1 / 12 points / 500 ms target — a **configuration that typically
times out**. One run shows how much merging saves and what to tune.
