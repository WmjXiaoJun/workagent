---
record: "0010"
case: firmware_budget
title: 固件资源预算测算
shape: 测算型
level: L3
date: 2026-10-10T21:52:55+08:00
date_short: 2026-10-10
verdict: 超限
input: examples/firmware_budget/input.json
input_digest: ba08fd26952dffbee74726120a83724275e378b4ff3f656bb6b01885b3a199b2
output_digest: 959acbbc1f9b2a228c6c19bd5d5c5e43c3482a1b47931ca179e1b868d590db90
evidence_id: fb3b3dfc56ca72ba
executor_digest: 5a8d24e97d90a73c596a21133d5b39c64ad319fd37b29a6c5987410e9c099004
package_version: 0.8.0
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
| 输出摘要 | `959acbbc1f9b2a22` |
| 执行体摘要 | `5a8d24e97d90a73c` |
| 证据包编号 | `fb3b3dfc56ca72ba` |
| 包版本 | 0.8.0 |
| 记录时间 | 2026-10-10T21:52:55+08:00 |

## 发现

- `[warn] FR-02` Flash 占用率 88.9%，超过告警阈值 85%（余量 114.0 KB）
- `[warn] FR-04` RAM 占用率 86.3%，超过告警阈值 80%（余量 17.5 KB）
- `[error] FR-05` 任务「通信任务」栈余量仅 16%（峰值 1290 / 分配 1536 B），低于安全下限 20%

## 复现

```bash
workagent run --case firmware_budget --input examples/firmware_budget/input.json .
workagent verify .
```
