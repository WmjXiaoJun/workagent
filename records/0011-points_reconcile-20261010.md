---
record: "0011"
case: points_reconcile
title: 积分账本一致性对账
shape: 核验型
level: L3
date: 2026-10-10T21:52:56+08:00
date_short: 2026-10-10
verdict: 不平
input: examples/points_reconcile/input.json
input_digest: d76d508f6ba1c7aa91d92f1a24d58547ed22e497a3a8e539b4db3da5f7435f11
output_digest: 0aa873ef1841e21d0e2316e8796255902e69ceb37e4afd2c1c337c4eeff55424
evidence_id: cf5d0bf5bd5680b7
executor_digest: a2f30cd84254b3719b7de5f3337912bae28206fbdca55b38b040fa33ba72f461
package_version: 0.8.0
---

# 0011 · 积分账本一致性对账

**结论**：对账判定「不平」：账本 7 条 / 账户 3 个，错误 2 项、告警 1 项；可用合计 2065.00，冻结合计 100.00

| 项 | 值 |
|---|---|
| 场景 | `points_reconcile`（核验型） |
| 复现等级 | L3 |
| 判定 | **不平** |
| 输入 | `examples/points_reconcile/input.json` |
| 输入摘要 | `d76d508f6ba1c7aa` |
| 输出摘要 | `0aa873ef1841e21d` |
| 执行体摘要 | `a2f30cd84254b371` |
| 证据包编号 | `cf5d0bf5bd5680b7` |
| 包版本 | 0.8.0 |
| 记录时间 | 2026-10-10T21:52:56+08:00 |

## 发现

- `[error] PR-03` 账户恒等式不成立：user=u_0003 可用+冻结=525.00 累计授予-累计消费=425.00
- `[error] PR-05` 已支付订单未入账：order=ap_A0005 user=u_0003 积分=500
- `[warn] PR-06` 预扣未结清：user=u_0003 reference=t_001 未结清笔数=1

## 复现

```bash
workagent run --case points_reconcile --input examples/points_reconcile/input.json .
workagent verify .
```
