#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""医疗 · 检验报告异常判定。

模型只做「把报告文本读成结构化指标」，参考区间判定全部由本文件确定性完成。
参考区间按性别区分；危急值单独标记。

【重要】参考区间为**示例值**，实际以各实验室报告单标注为准。见 README.md。
"""
import json, sys

# (项目, 性别, 下限, 上限, 单位)   sex: M/F/*
REFERENCE = [
    ("ALT",   "M",   9.0,  50.0, "U/L"),
    ("ALT",   "F",   7.0,  40.0, "U/L"),
    ("AST",   "M",  15.0,  40.0, "U/L"),
    ("AST",   "F",  13.0,  35.0, "U/L"),
    ("GLU",   "*",   3.9,   6.1, "mmol/L"),
    ("CREA",  "M",  57.0,  97.0, "umol/L"),
    ("CREA",  "F",  41.0,  73.0, "umol/L"),
    ("WBC",   "*",   3.5,   9.5, "10^9/L"),
    ("HGB",   "M", 130.0, 175.0, "g/L"),
    ("HGB",   "F", 115.0, 150.0, "g/L"),
    ("PLT",   "*", 125.0, 350.0, "10^9/L"),
    ("TC",    "*",   0.0,   5.2, "mmol/L"),
]
# 危急值 (项目, 低于, 高于)
CRITICAL = [
    ("GLU", 2.8, 22.2),
    ("WBC", 1.5, 30.0),
    ("PLT", 30.0, 1000.0),
    ("HGB", 60.0, 200.0),
]


def fail(reason):
    sys.stderr.write("BOUNDARY_VIOLATION: " + reason + "\n")
    sys.exit(2)


def main():
    if len(sys.argv) < 2:
        fail("缺少输入文件参数")
    with open(sys.argv[1], "r", encoding="utf-8") as fh:
        data = json.load(fh)
    payload, llm = (data["input"], data.get("llm") or {}) if isinstance(data.get("input"), dict) else (data, {})

    extract = llm.get("extract") if isinstance(llm, dict) else None
    rows, source = None, "direct"
    if isinstance(extract, dict) and isinstance(extract.get("items"), list) and extract["items"]:
        rows, source = extract["items"], "llm-extract"
    elif isinstance(payload.get("items"), list) and payload["items"]:
        rows = payload["items"]
    if not rows:
        fail("没有可判定的检验指标")

    sex = str(payload.get("sex", "")).upper()[:1]
    if sex not in ("M", "F"):
        fail("sex 必须为 M 或 F")

    findings, abnormal, critical = [], [], []
    for r in rows:
        r = r if isinstance(r, dict) else {}
        code = str(r.get("code", "")).upper()
        name = str(r.get("name", code))
        try:
            value = float(r.get("value"))
        except (TypeError, ValueError):
            findings.append({"level": "warn", "code": "LAB-02", "args": {"name": name},
                             "message": name + " 数值无法解析"})
            continue
        ref = None
        for c, sx, lo, hi, unit in REFERENCE:
            if c == code and sx in (sex, "*"):
                ref = (lo, hi, unit)
                break
        if ref is None:
            findings.append({"level": "info", "code": "LAB-03", "args": {"name": name},
                             "message": name + " 无参考区间"})
            continue
        lo, hi, unit = ref
        for c, clo, chi in CRITICAL:
            if c == code and (value < clo or value > chi):
                critical.append(code)
                findings.append({"level": "error", "code": "CRIT-01",
                                 "args": {"name": name, "value": str(value), "unit": unit,
                                          "lo": str(clo), "hi": str(chi)},
                                 "message": name + " 达危急值"})
        if value < lo:
            abnormal.append((code, "low"))
            findings.append({"level": "warn", "code": "REF-01",
                             "args": {"name": name, "value": str(value), "unit": unit, "ref": str(lo)},
                             "message": name + " 低于参考下限"})
        elif value > hi:
            abnormal.append((code, "high"))
            findings.append({"level": "warn", "code": "REF-02",
                             "args": {"name": name, "value": str(value), "unit": unit, "ref": str(hi)},
                             "message": name + " 高于参考上限"})

    if critical:
        verdict = "危急"
    elif abnormal:
        verdict = "异常"
    else:
        verdict = "未见异常"
    if source == "llm-extract":
        findings.insert(0, {"level": "info", "code": "LAB-01",
                            "args": {"count": str(len(rows))},
                            "message": "指标由模型抽取"})

    out = {
        "subject": str(payload.get("patient_id", "")) + " 检验报告（" + ("男" if sex == "M" else "女") + "）",
        "subject_code": "SUBJECT-M" if sex == "M" else "SUBJECT-F",
        "subject_args": {"patient": str(payload.get("patient_id", ""))},
        "verdict": verdict,
        # 危急值分支改成"总是出现"的常量格式 —— 句内条件片段是语言相关的，不该由执行体拼。
        "conclusion": "判定「" + verdict + "」：共 " + str(len(rows)) + " 项，异常 " + str(len(abnormal))
                      + " 项，危急值 " + str(len(set(critical))) + " 项",
        "conclusion_args": {"verdict": verdict, "items": len(rows),
                            "abnormal": len(abnormal), "critical": len(set(critical))},
        "findings": findings,
        "abnormal": [{"code": c, "direction": d} for c, d in abnormal],
        "critical": sorted(set(critical)),
        "metrics": {"item_count": len(rows), "abnormal_count": len(abnormal),
                    "critical_count": len(set(critical))},
        "confidence": {"level": "medium", "basis_code": "BASIS",
                       "basis": "参考区间为示例值，未与具体实验室报告单核对；判定逻辑为确定性规则"},
        "risks": [
            "**本结论不构成医学诊断**，仅供参考，必须由执业医师结合临床判断",
            "参考区间随实验室、方法学、年龄、妊娠状态变化，示例值可能与实际报告单不符",
            "模型抽取的数值存在误读风险，关键指标必须人工核对",
        ],
        "risks_i18n": [
            {"code": "01", "text": "本结论不构成医学诊断"},
            {"code": "02", "text": "参考区间随实验室、方法学、年龄、妊娠状态变化"},
            {"code": "03", "text": "模型抽取的数值存在误读风险"},
        ],
    }
    sys.stdout.write(json.dumps({"output": out}, ensure_ascii=True, sort_keys=True))
    return 0


if __name__ == "__main__":
    sys.exit(main())
