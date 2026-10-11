---
record: "0011"
case: points_reconcile
title: 积分账本一致性对账
shape: 核验型
level: L3
date: 2026-10-11T15:07:35+08:00
date_short: 2026-10-11
verdict: 不平
input: examples/points_reconcile/input.json
input_digest: d76d508f6ba1c7aa91d92f1a24d58547ed22e497a3a8e539b4db3da5f7435f11
output_digest: 473537534ec4d9ce5e2a68b43c2a7d1d2f42918b3c509d01a5b7bc7975533ab9
evidence_id: 0d3d3b0cd9a2af4d
executor_digest: f47392fc5d56b04e4f6bdf76a1b951968e7ae34bfc53c4014ad8648dd9e9ec29
package_version: 0.14.0
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
| 输出摘要 | `473537534ec4d9ce` |
| 执行体摘要 | `f47392fc5d56b04e` |
| 证据包编号 | `0d3d3b0cd9a2af4d` |
| 包版本 | 0.14.0 |
| 记录时间 | 2026-10-11T15:07:35+08:00 |

## 发现

- `[error] PR-03` 账户恒等式不成立：user=u_0003 可用+冻结=525.00 累计授予-累计消费=425.00
- `[error] PR-05` 已支付订单未入账：order=ap_A0005 user=u_0003 积分=500
- `[warn] PR-06` 预扣未结清：user=u_0003 reference=t_001 未结清笔数=1

## 复现

```bash
workagent run --case points_reconcile --input examples/points_reconcile/input.json .
workagent verify .
```
