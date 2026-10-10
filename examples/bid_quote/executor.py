#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
示例执行体：投标报价测算。

契约:
    argv[1] = 输入 JSON 路径。可能是：
      (a) 原始输入                              —— 纯确定性路径
      (b) 富化输入 {"input": {...}, "llm": {"extract": {...}}}
                                              —— 模型抽取后的路径
    stdout  : {"output": {...}}（纯 ASCII，便于跨平台复现）
    失败    : 非零退出码 + stderr 原因（对应 boundary.json 的 stop_condition）

设计要点:
    1. 模型只做「抽取」，判断与计算全部在本文件里确定性完成。
    2. 因此本执行体是纯函数：同样的中间表示一定得到同样的结论。
    3. 抽取本身不可复现 —— 那部分由 workagent 的 LLM 步骤录制与重放。
"""

import json
import sys


def fail(reason):
    sys.stderr.write("BOUNDARY_VIOLATION: " + reason + "\n")
    sys.exit(2)


def pick_payload(data):
    """兼容原始输入与富化输入两种形态。"""
    if isinstance(data, dict) and isinstance(data.get("input"), dict):
        return data["input"], data.get("llm") or {}
    return data, {}


def pick_items(payload, llm):
    """优先使用模型抽取结果；没有则退回确定性输入。"""
    extract = llm.get("extract") if isinstance(llm, dict) else None
    if isinstance(extract, dict):
        items = extract.get("line_items")
        if isinstance(items, list) and items:
            return items, "llm-extract"
    items = payload.get("line_items")
    if isinstance(items, list) and items:
        return items, "direct"
    return None, None


def num(value, field, name):
    if isinstance(value, bool):
        fail(field + " 类型不合法: " + str(name))
    if isinstance(value, (int, float)):
        return float(value)
    if isinstance(value, str):
        cleaned = value.replace(",", "").replace("，", "").replace("元", "").strip()
        try:
            return float(cleaned)
        except ValueError:
            fail(field + " 不是数字: " + str(name) + " -> " + repr(value))
    fail(field + " 缺失或类型不合法: " + str(name))


def main():
    if len(sys.argv) < 2:
        fail("缺少输入文件参数")
    with open(sys.argv[1], "r", encoding="utf-8") as fh:
        data = json.load(fh)

    payload, llm = pick_payload(data)
    items, source = pick_items(payload, llm)
    if items is None:
        fail("既没有 line_items 也没有可用的抽取结果")

    if len(items) > 500:
        fail("明细超过 500 行，超出 boundary.json 声明的适用范围")

    overhead_rate = payload.get("overhead_rate", 0.08)
    margin_rate = payload.get("margin_rate", 0.12)
    risk_buffer_rate = payload.get("risk_buffer_rate", 0.05)
    for name, value in (("overhead_rate", overhead_rate),
                        ("margin_rate", margin_rate),
                        ("risk_buffer_rate", risk_buffer_rate)):
        v = num(value, name, name)
        if v < 0 or v > 1:
            fail(name + " 超出 [0, 1] 区间")

    overhead_rate = num(overhead_rate, "overhead_rate", "overhead_rate")
    margin_rate = num(margin_rate, "margin_rate", "margin_rate")
    risk_buffer_rate = num(risk_buffer_rate, "risk_buffer_rate", "risk_buffer_rate")

    direct_cost = 0.0
    breakdown = []
    for item in items:
        if not isinstance(item, dict):
            fail("明细行不是对象")
        name = str(item.get("name", ""))
        qty = num(item.get("quantity"), "quantity", name)
        cost = num(item.get("unit_cost"), "unit_cost", name)
        if qty <= 0:
            fail("quantity 必须为正数: " + name)
        if cost < 0:
            fail("unit_cost 必须为非负数: " + name)
        amount = round(qty * cost, 6)
        direct_cost += amount
        breakdown.append({"name": name, "amount": round(amount, 2),
                          "category": str(item.get("category", "other"))})

    direct_cost = round(direct_cost, 6)
    overhead = round(direct_cost * overhead_rate, 6)
    loaded = direct_cost + overhead
    risk_buffer = round(loaded * risk_buffer_rate, 6)
    loaded = loaded + risk_buffer
    margin = round(loaded * margin_rate, 6)
    total_price = round(loaded + margin, 2)

    for row in breakdown:
        row["share"] = round(row["amount"] / direct_cost, 6) if direct_cost else 0.0

    if source == "llm-extract":
        level = "medium"
        basis = "明细由模型从自然语言抽取，存在识别误差；计算部分为确定性"
        extra_risk = "明细来自模型抽取，请对照原始描述复核后再签字"
    else:
        level = "high" if len(items) >= 3 else "medium"
        basis = "纯确定性计算，无外部数据依赖；上限受输入质量与假设 AS-001/AS-003 约束"
        extra_risk = "管理费费率假设（AS-001）失效时结论系统性偏低"

    result = {
        "subject": payload.get("project", ""),
        "verdict": "已测算",
        "findings": [
            {"level": "info", "code": "BQ-SRC",
             "message": "明细来源：" + source + "，共 " + str(len(items)) + " 项"},
        ],
        "project": payload.get("project", ""),
        "currency": payload.get("currency", "CNY"),
        "line_item_source": source,
        "direct_cost": round(direct_cost, 2),
        "overhead": round(overhead, 2),
        "risk_buffer": round(risk_buffer, 2),
        "margin": round(margin, 2),
        "total_price": total_price,
        "unit_price_breakdown": breakdown,
        "confidence": {"level": level, "basis": basis},
        "conclusion": "建议报价 " + str(payload.get("currency", "CNY")) + " "
                      + format(total_price, ",.2f")
                      + "（直接成本 " + format(round(direct_cost, 2), ",.2f")
                      + "，明细来源 " + source + "）",
        "risks": [
            "结论不构成报价承诺，需业务负责人签字确认",
            extra_risk,
            "未计入税费、汇率波动与工期延误",
        ],
    }
    sys.stdout.write(json.dumps({"output": result}, ensure_ascii=True, sort_keys=True))
    return 0


if __name__ == "__main__":
    sys.exit(main())
