#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""财务 · 报销单合规审核。

模型只做「把报销说明读成结构化明细」，合规判定全部由本文件确定性完成。
模型输出若有缺失，会被显式记为 finding，而不是被静默忽略。
"""
import json, sys
from datetime import date

CATEGORY_LIMITS = {"餐饮": 1000.0, "办公": 500.0, "住宿": 600.0}


def fail(reason):
    sys.stderr.write("BOUNDARY_VIOLATION: " + reason + "\n")
    sys.exit(2)


def num(v, field, name):
    if v is None:
        fail(field + " 缺失: " + str(name))
    try:
        return float(v)
    except (TypeError, ValueError):
        fail(field + " 不是数字: " + str(name))


def parse_date(s):
    try:
        y, m, d = str(s).split("-")
        return date(int(y), int(m), int(d))
    except Exception:
        return None


def main():
    if len(sys.argv) < 2:
        fail("缺少输入文件参数")
    with open(sys.argv[1], "r", encoding="utf-8") as fh:
        data = json.load(fh)
    payload, llm = (data["input"], data.get("llm") or {}) if isinstance(data.get("input"), dict) else (data, {})

    extract = llm.get("extract") if isinstance(llm, dict) else None
    items = None
    source = "direct"
    if isinstance(extract, dict) and isinstance(extract.get("items"), list) and extract["items"]:
        items = extract["items"]
        source = "llm-extract"
    elif isinstance(payload.get("items"), list) and payload["items"]:
        items = payload["items"]
    if not items:
        fail("既没有 items 也没有可用的抽取结果")

    policy = payload.get("policy") or {}
    single_limit = float(policy.get("single_limit", 5000))
    max_age_days = int(policy.get("max_age_days", 30))
    submit = parse_date(payload.get("submit_date"))

    findings = []
    total = 0.0
    categories = {}
    for idx, it in enumerate(items):
        it = it if isinstance(it, dict) else {}
        name = str(it.get("name") or it.get("description") or ("第%d项" % (idx + 1)))
        cat = str(it.get("category") or "未分类")
        amount = num(it.get("amount"), "amount", name)
        if amount < 0:
            fail("amount 为负: " + name)
        total += amount
        categories[cat] = categories.get(cat, 0.0) + amount

        if amount > single_limit:
            findings.append({"level": "error", "code": "EXP-02",
                             "args": {"name": name, "amount": format(amount, ",.2f"),
                                      "limit": format(single_limit, ",.2f")},
                             "message": name + " 超过单笔上限"})
        limit = CATEGORY_LIMITS.get(cat)
        if limit is not None and amount > limit:
            findings.append({"level": "error", "code": "EXP-03",
                             "args": {"name": name, "category": cat, "amount": format(amount, ",.2f"),
                                      "limit": format(limit, ",.2f")},
                             "message": name + " 超过类别上限"})
        if cat not in ("交通", "住宿", "餐饮", "办公", "其他", "未分类"):
            findings.append({"level": "warn", "code": "EXP-05",
                             "args": {"name": name, "category": cat},
                             "message": name + " 类别不在白名单"})
        if not it.get("invoice_no"):
            findings.append({"level": "warn", "code": "EXP-06",
                             "args": {"name": name},
                             "message": name + " 缺少发票号"})
        d = parse_date(it.get("date"))
        if submit and d:
            delta = (submit - d).days
            if delta < 0:
                # 同一个 code 不能有两种文案（文案表是 code → 模板），所以拆码。
                findings.append({"level": "error", "code": "EXP-04",
                                 "args": {"name": name},
                                 "message": name + " 发生日期晚于提交日期"})
            elif delta > max_age_days:
                findings.append({"level": "error", "code": "EXP-04B",
                                 "args": {"name": name, "days": str(delta), "limit": str(max_age_days)},
                                 "message": name + " 超过报销时限"})

    claimed = payload.get("claimed_total")
    if claimed is not None:
        claimed = num(claimed, "claimed_total", "claimed_total")
        if abs(claimed - round(total, 2)) > 0.01:
            findings.append({"level": "error", "code": "EXP-01",
                             "args": {"claimed": format(claimed, ",.2f"), "total": format(total, ",.2f")},
                             "message": "申报总额与明细合计不一致"})

    if source == "llm-extract":
        notes = extract.get("notes") if isinstance(extract, dict) else None
        if notes:
            findings.append({"level": "warn", "code": "EXP-07",
                             "args": {"notes": str(notes)},
                             "message": "模型抽取标注了不确定项"})

    errors = [f for f in findings if f["level"] == "error"]
    warns = [f for f in findings if f["level"] == "warn"]
    verdict = "退回" if errors else ("需人工复核" if warns else "通过")

    out = {
        "subject": str(payload.get("claim_id", "")) + " " + str(payload.get("submitter", "")) + " 报销单",
        "verdict": verdict,
        "conclusion": "审核判定「" + verdict + "」：明细 " + str(len(items)) + " 项合计 "
                      + format(round(total, 2), ",.2f") + " 元；错误 " + str(len(errors)) + " 项、告警 " + str(len(warns)) + " 项",
        # 结论句的参数化事实：渲染层据此生成中/英文，执行体本身不含语言。
        "conclusion_args": {"verdict": verdict, "items": len(items),
                            "total": format(round(total, 2), ",.2f"),
                            "errors": len(errors), "warns": len(warns)},
        "findings": findings if findings else [{"level": "info", "code": "EXP-OK", "message": "未发现违规项"}],
        "line_item_source": source,
        "metrics": {"item_count": len(items), "total": round(total, 2),
                    "categories": {k: round(v, 2) for k, v in sorted(categories.items())},
                    "errors": len(errors), "warnings": len(warns)},
        "confidence": {"level": "medium" if source == "llm-extract" else "high",
                       "basis": ("明细由模型从报销说明抽取，存在识别误差；合规判定为确定性规则"
                                 if source == "llm-extract" else "明细为结构化输入，判定为确定性规则")},
        "risks": [
            "结论不构成付款依据，需业务负责人签字确认",
            "发票真伪与税额未做校验",
            "类别限额为示例值，需替换为企业实际制度",
        ],
    }
    sys.stdout.write(json.dumps({"output": out}, ensure_ascii=True, sort_keys=True))
    return 0


if __name__ == "__main__":
    sys.exit(main())
