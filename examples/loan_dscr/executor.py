#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""项目贷款偿债能力（DSCR）测算。

DSCR = (净利润 + 折旧 + 摊销 + 利息支出) / (本金摊还 + 利息支出)
常用阈值（示例，可配置）：>=1.25 通过；1.10~1.25 需复核；<1.10 不予通过。
另计利息保障倍数 = EBIT / 利息支出。
纯确定性，无 LLM（L3）。
"""
import json, sys


def fail(reason):
    sys.stderr.write("BOUNDARY_VIOLATION: " + reason + "\n")
    sys.exit(2)


def num(v, name):
    try:
        return float(v)
    except (TypeError, ValueError):
        fail(name + " 不是数字")


def main():
    if len(sys.argv) < 2:
        fail("缺少输入文件参数")
    with open(sys.argv[1], "r", encoding="utf-8") as fh:
        data = json.load(fh)
    if isinstance(data.get("input"), dict):
        data = data["input"]

    profit = num(data.get("net_profit"), "net_profit")
    dep = num(data.get("depreciation", 0), "depreciation")
    amo = num(data.get("amortization", 0), "amortization")
    interest = num(data.get("interest_expense"), "interest_expense")
    principal = num(data.get("principal_repayment"), "principal_repayment")
    tax_rate = float(data.get("tax_rate", 0.25))
    threshold_ok = float(data.get("dscr_min", 1.25))
    threshold_review = float(data.get("dscr_review", 1.10))

    if principal + interest <= 0:
        fail("本金摊还与利息合计为 0，无法计算覆盖倍数")
    if interest <= 0:
        fail("利息支出必须为正，否则 DSCR 退化")

    cfads = profit + dep + amo + interest
    dscr = round(cfads / (principal + interest), 4)
    ebit = profit / (1 - tax_rate) if tax_rate < 1 else profit
    icr = round(ebit / interest, 4)

    findings, verdict = [], ""
    if dscr >= threshold_ok:
        verdict = "通过"
        findings.append({"level": "info", "code": "DSCR-OK",
                         "args": {"dscr": format(dscr, ".2f"), "threshold": format(threshold_ok, ".2f")},
                         "message": "DSCR 达到阈值"})
    elif dscr >= threshold_review:
        verdict = "需复核"
        findings.append({"level": "warn", "code": "DSCR-REVIEW",
                         "args": {"dscr": format(dscr, ".2f"), "ok": format(threshold_ok, ".2f"),
                                  "review": format(threshold_review, ".2f")},
                         "message": "DSCR 处于复核区间"})
    else:
        verdict = "不予通过"
        findings.append({"level": "error", "code": "DSCR-FAIL",
                         "args": {"dscr": format(dscr, ".2f"), "floor": format(threshold_review, ".2f")},
                         "message": "DSCR 低于下限"})
    if icr < 2.0:
        findings.append({"level": "warn", "code": "ICR-LOW",
                         "args": {"icr": format(icr, ".2f"), "threshold": "2.00"},
                         "message": "利息保障倍数偏低"})

    out = {
        "subject": str(data.get("project", "")) + " 偿债能力测算",
        "verdict": verdict,
        "conclusion": "DSCR = " + format(dscr, ".2f") + "，判定「" + verdict + "」；可用于偿债的现金流 "
                      + format(cfads, ",.2f") + "，当年还本付息 " + format(principal + interest, ",.2f"),
        "conclusion_args": {"dscr": format(dscr, ".2f"), "verdict": verdict,
                            "cfads": format(cfads, ",.2f"),
                            "debt_service": format(principal + interest, ",.2f")},
        "findings": findings,
        "metrics": {"cfads": round(cfads, 2), "debt_service": round(principal + interest, 2),
                    "dscr": dscr, "interest_coverage": icr},
        "confidence": {"level": "high",
                       "basis": "纯确定性公式；阈值来自贷款审批惯例，可在输入中覆盖"},
        "risks": [
            "未考虑现金流季节性波动与建设期超支",
            "净利润为会计口径，与经营现金流可能背离",
            "阈值 DSCR>=" + format(threshold_ok, ".2f") + " 为示例惯例，各行政策不同",
        ],
    }
    sys.stdout.write(json.dumps({"output": out}, ensure_ascii=True, sort_keys=True))
    return 0


if __name__ == "__main__":
    sys.exit(main())