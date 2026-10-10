---
record: "0010"
case: firmware_budget
title: 固件资源预算测算
shape: 测算型
level: L3
date: 2026-10-11T00:03:34+08:00
date_short: 2026-10-11
verdict: 超限
input: examples/firmware_budget/input.json
input_digest: ba08fd26952dffbee74726120a83724275e378b4ff3f656bb6b01885b3a199b2
output_digest: c661df69064bd8e62959b2a2df779649f90a7df4f686ef96ba0185ab2399a571
evidence_id: e336b71b9e68ed04
executor_digest: f27a90a40a9d223fce1bb704eb324041139ebd3897d640d9e62b460e5387acf7
package_version: 0.13.0
---

# 0010 · 固件资源预算测算

**结论**：判定「超限」：Flash 910.0/1024.0 KB（88.9%），RAM 110.5/128.0 KB（86.3%）；错误 1 项、告警 2 项

| 项 | 值 |
|---|---|
| 场景 | `firmware_budget`（测算型） |
| 复现等级 | L3 |
| 判定 | **超限** |
| 输入 | `examples/firmware_budget/input.json` |
| 输入摘要 | `ba08fd26952dffbe` |
| 输出摘要 | `c661df69064bd8e6` |
| 执行体摘要 | `f27a90a40a9d223f` |
| 证据包编号 | `e336b71b9e68ed04` |
| 包版本 | 0.13.0 |
| 记录时间 | 2026-10-11T00:03:34+08:00 |

## 发现

- `[warn] FR-02` Flash 占用率 88.9%，超过告警阈值 85%（余量 114.0 KB）
- `[warn] FR-04` RAM 占用率 86.3%，超过告警阈值 80%（余量 17.5 KB）
- `[error] FR-05` 任务「通信任务」栈余量仅 16%（峰值 1290 / 分配 1536 B），低于安全下限 20%

## 复现

```bash
workagent run --case firmware_budget --input examples/firmware_budget/input.json .
workagent verify .
```
