#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
示例执行体：投标报价测算。

契约:
    输入: argv[1] = 输入 JSON 路径
    输出: stdout 打印 {"output": {...}}  —— 纯 ASCII，便于跨平台与跨编码复现
    失败: 退出码非 0，并打印原因 (对应 boundary.json 的 stop_condition)

设计:
    纯确定性计算，无随机、无网络、无时间依赖 —— 因此可复现。
    如需接入 LLM，请单开步骤，并在 attest/closure.json 登记模型闭包摘要。
"""

import json
import sys


def fail(reason):
    sys.stderr.write("BOUNDARY_VIOLATION: " + reason + "\n")
    sys.exit(2)


def main():
    if len(sys.argv) < 2:
        fail("缺少输入文件参数")
    with open(sys.argv[1], "r", encoding="utf-8") as fh:
        data = json.load(fh)

    items = data.get("line_items")
    if not isinstance(items, list) or not items:
        fail("line_items 为空或不是数组")
    if len(items) > 500:
        fail("line_items 超过 500 行，超出 boundary.json 声明的适用范围")

    overhead_rate = data.get("overhead_rate", 0.08)
    margin_rate = data.get("margin_rate", 0.12)
    risk_buffer_rate = data.get("risk_buffer_rate", 0.05)
    for name, value in (("overhead_rate", overhead_rate),
                        ("margin_rate", margin_rate),
                        ("risk_buffer_rate", risk_buffer_rate)):
        if not isinstance(value, (int, float)) or value < 0 or value > 1:
            fail(name + " 超出 [0, 1] 区间")

    direct_cost = 0.0
    breakdown = []
    for item in items:
        qty = item.get("quantity")
        cost = item.get("unit_cost")
        if not isinstance(qty, (int, float)) or qty <= 0:
            fail("quantity 必须为正数: " + str(item.get("name")))
        if not isinstance(cost, (int, float)) or cost < 0:
            fail("unit_cost 必须为非负数: " + str(item.get("name")))
        amount = round(float(qty) * float(cost), 6)
        direct_cost += amount
        breakdown.append({"name": item.get("name", ""), "amount": round(amount, 2)})

    direct_cost = round(direct_cost, 6)
    overhead = round(direct_cost * overhead_rate, 6)
    loaded = direct_cost + overhead
    risk_buffer = round(loaded * risk_buffer_rate, 6)
    loaded = loaded + risk_buffer
    margin = round(loaded * margin_rate, 6)
    total_price = round(loaded + margin, 2)

    for row in breakdown:
        row["share"] = round(row["amount"] / direct_cost, 6) if direct_cost else 0.0

    confidence_level = "high" if len(items) >= 3 else "medium"
    result = {
        "project": data.get("project", ""),
        "currency": data.get("currency", "CNY"),
        "direct_cost": round(direct_cost, 2),
        "overhead": round(overhead, 2),
        "risk_buffer": round(risk_buffer, 2),
        "margin": round(margin, 2),
        "total_price": total_price,
        "unit_price_breakdown": breakdown,
        "confidence": {
            "level": confidence_level,
            "basis": "纯确定性计算，无外部数据依赖；上限受输入质量与假设 AS-001/AS-003 约束"
        },
        "conclusion": "建议报价 " + str(data.get("currency", "CNY")) + " " + format(total_price, ",.2f")
                      + "（直接成本 " + format(round(direct_cost, 2), ",.2f")
                      + "，利润率 " + format(margin_rate * 100, ".0f") + "%）",
        "risks": [
            "结论不构成报价承诺，需业务负责人签字确认",
            "管理费费率假设（AS-001）失效时结论系统性偏低",
            "未计入税费、汇率波动与工期延误"
        ]
    }
    sys.stdout.write(json.dumps({"output": result}, ensure_ascii=True, sort_keys=True))
    return 0


if __name__ == "__main__":
    sys.exit(main())
