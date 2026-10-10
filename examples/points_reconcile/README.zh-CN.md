# 积分账本一致性对账

**形态**：核验型 ｜ **复现等级**：L3（纯确定性，无 LLM）

## 规则从哪来

**不是我想出来的，是在真实生产库上验证出来的。**

先拿下面七条不变量去跑一个真实运行的计费库（21 条账本 / 20 个账户 / 8 笔订单）：
**违反数 0**。确认规则正确后，才把它固化成可复现场景。

> 示例数据是**合成的**——结构照抄真实库，行不是。真实账本含用户与金额，不入公开仓库。

## 七项不变量

| 代号 | 规则 | 级别 |
|---|---|---|
| PR-01 | 账本链式累计：`balance_after[i] == balance_after[i-1] + signed_amount` | error |
| PR-02 | 账户 `available_points` == 该用户账本末笔 `balance_after` | error |
| PR-03 | `available + frozen == lifetime_granted - lifetime_spent` | error |
| PR-04 | `idempotency_key` 全局唯一 | error |
| PR-05 | 已支付订单必须有对应入账 entry | error |
| PR-06 | reserve 必须被 capture 或 refund 结清（预扣不能悬空） | warn |
| PR-07 | 金额符号与 `entry_type` 语义一致 | warn |

### reserve / capture / refund 的坑

真实账本的 `entry_type` 不止 grant/spend，还有一整套预扣-结算-退款流：

```text
计入余额增加: grant, admin_grant, capture, refund, sub2api_capture, sub2api_refund
计入余额减少: spend, reserve, sub2api_reserve
```

**PR-03 在这个语义下最容易误判。** reserve 时 `available` 减少、`frozen` 等量增加，
所以 `available + frozen` 应保持不变；如果 `lifetime_spent` 也把 reserve 计进去，
就会出现"同一笔预扣被计了两次"——恒等式不成立。示例数据里 u_0003 正是这一类错误。

## 实测输出

```text
对账判定「不平」：账本 7 条 / 账户 3 个，错误 2 项、告警 1 项

[error] PR-03 账户恒等式不成立：user=u_0003 可用+冻结=525.00 累计授予-累计消费=425.00
[error] PR-05 已支付订单未入账：order=ap_A0005 user=u_0003 积分=500
[warn]  PR-06 预扣未结清：user=u_0003 reference=t_001 未结清笔数=1
```

## 运行

```bash
workagent run --case points_reconcile --input examples/points_reconcile/input.json .
workagent verify .
```

## 边界

**只做自洽性对账，不校验业务语义。** 它不会告诉你"这笔 500 积分该不该给"，
只会告诉你"这笔预扣没结清、这个订单付了钱没入账"。**后者才是资损。**
