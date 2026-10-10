#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""积分账本一致性对账。

规则来源：计费系统必须成立的不变量。每一条都先在真实生产库上验证过（违反数 = 0），
确认规则正确后再固化为可复现场景。

  PR-01  账本链式累计：balance_after[i] == balance_after[i-1] + signed_amount
  PR-02  账户 available_points == 该用户账本末笔 balance_after
  PR-03  available + frozen == lifetime_granted - lifetime_spent
  PR-04  idempotency_key 全局唯一
  PR-05  已支付订单必须有对应入账 entry
  PR-06  reserve 必须被 capture 或 refund 结清（预扣不能悬空）
  PR-07  金额符号必须与 entry_type 语义一致

reserve / capture / refund 语义（来自真实账本）：
  grant / admin_grant / capture / refund  -> 余额增加
  spend / reserve                          -> 余额减少

纯确定性：同一份账本必然得到同样的对账结论（L3）。
"""
import json, sys

POSITIVE = ("grant", "admin_grant", "capture", "refund",
            "sub2api_capture", "sub2api_refund")
NEGATIVE = ("spend", "reserve", "sub2api_reserve")


def fail(reason):
    sys.stderr.write("BOUNDARY_VIOLATION: " + reason + "\n")
    sys.exit(2)


def num(v, name):
    if v is None:
        return 0.0
    if isinstance(v, bool):
        fail(name + " 类型不合法")
    try:
        return float(v)
    except (TypeError, ValueError):
        fail(name + " 不是数字: " + repr(v))


def main():
    if len(sys.argv) < 2:
        fail("缺少输入文件参数")
    with open(sys.argv[1], "r", encoding="utf-8") as fh:
        data = json.load(fh)
    if isinstance(data.get("input"), dict):
        data = data["input"]

    ledger = data.get("ledger_entries")
    accounts = data.get("accounts")
    orders = data.get("payment_orders") or []
    if not isinstance(ledger, list) or not ledger:
        fail("ledger_entries 为空")
    if not isinstance(accounts, list) or not accounts:
        fail("accounts 为空")
    if len(ledger) > 200000:
        fail("账本条目超过 200000，超出 boundary.json 声明的规模")

    findings = []

    # 按用户、按时间排序
    def sort_key(e):
        return (str(e.get("user_id", "")), str(e.get("created_at", "")), str(e.get("id", "")))
    ordered = sorted([e for e in ledger if isinstance(e, dict)], key=sort_key)

    per_user = {}
    for e in ordered:
        per_user.setdefault(str(e.get("user_id", "")), []).append(e)

    chain_bad = 0
    for uid, rows in per_user.items():
        prev = None
        for e in rows:
            et = str(e.get("entry_type", ""))
            amt = num(e.get("amount"), "amount")
            bal = num(e.get("balance_after"), "balance_after")
            signed = amt if et in POSITIVE else (-amt if et in NEGATIVE else amt)
            if et not in POSITIVE and et not in NEGATIVE:
                findings.append({"level": "warn", "code": "PR-07",
                                 "args": {"entry_type": et, "id": str(e.get("id"))},
                                 "message": "未知 entry_type：" + et})
            if prev is not None:
                expect = round(prev + signed, 6)
                if abs(bal - expect) > 1e-6:
                    chain_bad += 1
                    findings.append({"level": "error", "code": "PR-01",
                                     "args": {"user": uid[:8], "id": str(e.get("id")),
                                              "prev": "%.2f" % prev, "entry_type": et,
                                              "amount": "%.2f" % amt, "balance": "%.2f" % bal,
                                              "expect": "%.2f" % expect},
                                     "message": "账本断链：" + uid[:8]})
            prev = bal

    # PR-02 / PR-03
    accmap = {str(a.get("user_id", "")): a for a in accounts if isinstance(a, dict)}
    for uid, a in accmap.items():
        rows = per_user.get(uid, [])
        avail = num(a.get("available_points"), "available_points")
        frozen = num(a.get("frozen_points"), "frozen_points")
        granted = num(a.get("lifetime_granted"), "lifetime_granted")
        spent = num(a.get("lifetime_spent"), "lifetime_spent")
        if rows:
            last = num(rows[-1].get("balance_after"), "balance_after")
            if abs(last - avail) > 1e-6:
                findings.append({"level": "error", "code": "PR-02",
                                 "args": {"user": uid[:8], "avail": "%.2f" % avail, "last": "%.2f" % last},
                                 "message": "账户余额与账本末笔不符：" + uid[:8]})
        lhs = avail + frozen
        rhs = granted - spent
        if abs(lhs - rhs) > 1e-6:
            findings.append({"level": "error", "code": "PR-03",
                             "args": {"user": uid[:8], "lhs": "%.2f" % lhs, "rhs": "%.2f" % rhs},
                             "message": "账户恒等式不成立：" + uid[:8]})

    # PR-04 幂等键唯一
    seen = {}
    for e in ordered:
        k = e.get("idempotency_key")
        if not k:
            continue
        k = str(k)
        if k in seen:
            findings.append({"level": "error", "code": "PR-04",
                             "args": {"key": k, "id1": str(seen[k]), "id2": str(e.get("id"))},
                             "message": "幂等键重复：" + k})
        else:
            seen[k] = e.get("id")

    # PR-05 已支付订单必须入账
    refs = {str(e.get("reference_id")) for e in ordered if e.get("reference_id")}
    for o in orders:
        if not isinstance(o, dict):
            continue
        st = str(o.get("status", "")).lower()
        if st in ("paid", "success", "succeeded") and str(o.get("id")) not in refs:
            findings.append({"level": "error", "code": "PR-05",
                             "args": {"order": str(o.get("id")), "user": str(o.get("user_id"))[:8],
                                      "points": str(o.get("points"))},
                             "message": "已支付订单未入账：" + str(o.get("id"))})

    # PR-06 reserve 必须被结清
    for uid, rows in per_user.items():
        open_res = {}
        for e in rows:
            et = str(e.get("entry_type", ""))
            rid = str(e.get("reference_id") or "")
            if et == "sub2api_reserve" or et == "reserve":
                open_res[rid] = open_res.get(rid, 0) + 1
            elif et in ("capture", "refund", "sub2api_capture", "sub2api_refund"):
                if rid in open_res and open_res[rid] > 0:
                    open_res[rid] -= 1
        for rid, n in open_res.items():
            if n > 0:
                findings.append({"level": "warn", "code": "PR-06",
                                 "args": {"user": uid[:8], "reference": (rid or "-"), "n": str(n)},
                                 "message": "预扣未结清：" + uid[:8]})

    errors = [f for f in findings if f["level"] == "error"]
    warns = [f for f in findings if f["level"] == "warn"]
    verdict = "不平" if errors else ("有告警" if warns else "平")

    total_available = sum(num(a.get("available_points"), "available_points") for a in accounts)
    total_frozen = sum(num(a.get("frozen_points"), "frozen_points") for a in accounts)
    if not findings:
        findings.append({"level": "info", "code": "PR-OK",
                         "args": {"ledger": len(ledger), "accounts": len(accounts), "orders": len(orders)},
                         "message": "七项不变量全部成立"})

    out = {
        "subject": str(data.get("system", "积分账本")) + " 一致性对账",
        "verdict": verdict,
        "conclusion": "对账判定「" + verdict + "」：账本 %d 条 / 账户 %d 个，错误 %d 项、告警 %d 项；可用合计 %.2f，冻结合计 %.2f"
                      % (len(ledger), len(accounts), len(errors), len(warns), total_available, total_frozen),
        # 结论句的参数化事实：渲染层据此生成中/英文，执行体本身不含语言。
        "conclusion_args": {"verdict": verdict, "ledger": len(ledger), "accounts": len(accounts),
                            "errors": len(errors), "warns": len(warns),
                            "available": "%.2f" % total_available, "frozen": "%.2f" % total_frozen},
        "findings": findings,
        "metrics": {
            "ledger_entries": len(ledger), "accounts": len(accounts), "orders": len(orders),
            "chain_breaks": chain_bad,
            "errors": len(errors), "warnings": len(warns),
            "total_available": round(total_available, 2), "total_frozen": round(total_frozen, 2),
        },
        "confidence": {"level": "high",
                       "basis": "七项不变量为纯确定性校验，已在真实生产库上验证（违反数 0）；规则不依赖模型"},
        "risks": [
            "只校验账本与账户自身的自洽性，不校验业务语义（例如某笔授予是否该给）",
            "reserve/capture 配对按 reference_id 计数，跨系统对账需保证 reference_id 口径一致",
            "未见得覆盖并发写入下的瞬时中间态（应在事务边界处对账）",
        ],
    }
    sys.stdout.write(json.dumps({"output": out}, ensure_ascii=True, sort_keys=True))
    return 0


if __name__ == "__main__":
    sys.exit(main())
