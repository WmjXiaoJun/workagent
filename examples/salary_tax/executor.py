#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""人力资源 · 工资薪金个人所得税（累计预扣法）测算。

规则来源：中国《个人所得税法》居民个人综合所得预扣预缴，7 级超额累进。
  累计应纳税所得额 = 累计收入 - 5000×月数 - 累计专项扣除(三险一金) - 累计专项附加扣除
  本期应预扣 = 累计应纳税额 - 累计已预扣
纯确定性，无 LLM。同样输入必然同样输出（L3）。
"""
import json, sys

# 7 级累进预扣率表：(级距上限, 税率, 速算扣除数)
BRACKETS = [
    (36000, 0.03, 0),
    (144000, 0.10, 2520),
    (300000, 0.20, 16920),
    (420000, 0.25, 31920),
    (660000, 0.30, 52920),
    (960000, 0.35, 85920),
    (float("inf"), 0.45, 181920),
]
BASIC_DEDUCTION_PER_MONTH = 5000.0


def fail(reason):
    sys.stderr.write("BOUNDARY_VIOLATION: " + reason + "\n")
    sys.exit(2)


def num(v, field, name):
    try:
        return float(v)
    except (TypeError, ValueError):
        fail(field + " 不是数字: " + str(name))


def cumulative_tax(taxable):
    if taxable <= 0:
        return 0.0
    for cap, rate, quick in BRACKETS:
        if taxable <= cap:
            return round(taxable * rate - quick, 2)
    return 0.0


def main():
    if len(sys.argv) < 2:
        fail("缺少输入文件参数")
    with open(sys.argv[1], "r", encoding="utf-8") as fh:
        data = json.load(fh)
    if isinstance(data.get("input"), dict):
        data = data["input"]

    months = data.get("months")
    if not isinstance(months, list) or not months:
        fail("months 为空")
    if len(months) > 12:
        fail("months 超过 12，超出 boundary.json 声明的范围")

    rows, cum_income, cum_social, cum_special, cum_tax = [], 0.0, 0.0, 0.0, 0.0
    for i, m in enumerate(months):
        m = m if isinstance(m, dict) else {}
        income = num(m.get("salary"), "salary", i)
        social = num(m.get("social_insurance", 0), "social_insurance", i)
        special = num(m.get("special_deduction", 0), "special_deduction", i)
        if income < 0:
            fail("salary 为负: " + str(i))
        cum_income += income
        cum_social += social
        cum_special += special
        deduction = BASIC_DEDUCTION_PER_MONTH * (i + 1)
        taxable = round(cum_income - deduction - cum_social - cum_special, 2)
        if taxable < 0:
            taxable = 0.0
        due = cumulative_tax(taxable)
        period = round(due - cum_tax, 2)
        if period < 0:
            period = 0.0
        cum_tax = round(cum_tax + period, 2)
        rows.append({"month": i + 1, "income": round(income, 2), "cum_taxable": taxable,
                     "cum_tax": cum_tax, "period_tax": period,
                     "net": round(income - social - period, 2)})

    total_income = round(cum_income, 2)
    total_tax = round(cum_tax, 2)
    eff = round(total_tax / total_income, 6) if total_income else 0.0
    out = {
        "subject": str(data.get("employee", "")) + " " + str(data.get("tax_year", "")) + " 年度个税测算",
        "verdict": "已测算",
        "conclusion": "全年应纳个税合计 " + format(total_tax, ",.2f") + " 元（税前收入 "
                      + format(total_income, ",.2f") + " 元，实际税负 " + format(eff * 100, ".2f") + "%）",
        "conclusion_args": {"total_tax": format(total_tax, ",.2f"),
                            "total_income": format(total_income, ",.2f"),
                            "rate": format(eff * 100, ".2f")},
        "findings": [
            {"level": "info", "code": "TAX-YTD",
             "args": {"total_tax": format(total_tax, ",.2f")},
             "message": "按累计预扣法逐月计算，年末累计应纳 " + format(total_tax, ",.2f") + " 元"}
        ],
        "monthly": rows,
        "metrics": {"total_income": total_income, "total_tax": total_tax,
                    "effective_rate": eff, "months": len(rows)},
        "confidence": {"level": "high",
                       "basis": "规则取自《个人所得税法》7 级累进预扣率表，纯确定性计算，无外部数据依赖"},
        "risks": [
            "未计入年终奖单独计税、劳务报酬、股权激励等特殊项目",
            "专项附加扣除以输入值为准，未做资格校验",
            "各地社保基数上下限差异未体现",
        ],
    }
    sys.stdout.write(json.dumps({"output": out}, ensure_ascii=True, sort_keys=True))
    return 0


if __name__ == "__main__":
    sys.exit(main())