---
record: "0010"
case: firmware_budget
title: 固件资源预算测算
shape: 测算型
level: L3
date: 2026-10-11T15:07:35+08:00
date_short: 2026-10-11
verdict: 超限
input: examples/firmware_budget/input.json
input_digest: ba08fd26952dffbee74726120a83724275e378b4ff3f656bb6b01885b3a199b2
output_digest: 64309923cad2312043975197b50a05124f3754c3d225fe63d629438638a412d1
evidence_id: d22b9cb3a2e7a5a6
executor_digest: dc0c87d83fd9591662d8298d8705bbca2a5a71b07b627dd6de42262eae4ec1ce
package_version: 0.14.0
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
| 输出摘要 | `64309923cad23120` |
| 执行体摘要 | `dc0c87d83fd95916` |
| 证据包编号 | `d22b9cb3a2e7a5a6` |
| 包版本 | 0.14.0 |
| 记录时间 | 2026-10-11T15:07:35+08:00 |

## 发现

- `[warn] FR-02` Flash 占用率 88.9%，超过告警阈值 85%（余量 114.0 KB）
- `[warn] FR-04` RAM 占用率 86.3%，超过告警阈值 80%（余量 17.5 KB）
- `[error] FR-05` 任务「通信任务」栈余量仅 16%（峰值 1290 / 分配 1536 B），低于安全下限 20%

## 复现

```bash
workagent run --case firmware_budget --input examples/firmware_budget/input.json .
workagent verify .
```
