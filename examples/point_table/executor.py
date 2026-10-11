#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""工控 / 上位机 · 通信点表一致性核对。

模型只做「把点表文本读成结构化点位」，一致性检查全部由本文件确定性完成。
"""
import json, sys

# data_type -> 占用的保持寄存器个数
REG_COUNT = {"int16": 1, "uint16": 1, "int32": 2, "uint32": 2, "float32": 2, "float64": 4}
VALID_FC = (1, 2, 3, 4, 6, 16)


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
    points, source = None, "direct"
    if isinstance(extract, dict) and isinstance(extract.get("points"), list) and extract["points"]:
        points, source = extract["points"], "llm-extract"
    elif isinstance(payload.get("points"), list) and payload["points"]:
        points = payload["points"]
    if not points:
        fail("没有可核对的点位")

    findings, spans = [], []
    for p in points:
        p = p if isinstance(p, dict) else {}
        name = str(p.get("name", "")).strip()
        fc = p.get("function_code")
        addr = p.get("address")
        dtype = str(p.get("data_type", "")).lower()
        try:
            addr = int(addr)
        except (TypeError, ValueError):
            # 同一个 code 不能有两种文案，拆码。
            findings.append({"level": "error", "code": "PT-04", "args": {"name": name, "addr": repr(p.get("address"))},
                             "message": name + " 地址非法"})
            continue
        if addr < 0 or addr > 65535:
            findings.append({"level": "error", "code": "PT-04B", "args": {"name": name, "addr": str(addr)},
                             "message": name + " 地址越界"})
            continue
        # Modbus 参考地址归一化：4xxxxx=保持寄存器、3xxxxx=输入寄存器、1xxxxx=离散输入、0xxxxx=线圈
        # 现场点表普遍写 40001 这种"参考号"，实际偏移是 %10000，这一步不归一化会造成大面积误判。
        ref_kind = ""
        if 40001 <= addr <= 49999:
            ref_kind, addr = "holding", addr - 40001
        elif 30001 <= addr <= 39999:
            ref_kind, addr = "input", addr - 30001
        elif 10001 <= addr <= 19999:
            ref_kind, addr = "discrete", addr - 10001
        elif 1 <= addr <= 9999:
            ref_kind, addr = "coil", addr - 1
        if fc not in VALID_FC:
            findings.append({"level": "error", "code": "PT-06", "args": {"name": name, "fc": str(fc)},
                             "message": name + " 功能码非法"})
        n = REG_COUNT.get(dtype)
        if n is None:
            findings.append({"level": "warn", "code": "PT-02", "args": {"name": name, "dtype": dtype},
                             "message": name + " 数据类型未知"})
            n = 1
        spans.append((fc, addr, addr + n - 1, name, dtype))
        lo, hi = p.get("scale_min"), p.get("scale_max")
        if lo is not None and hi is not None:
            try:
                if float(lo) >= float(hi):
                    findings.append({"level": "error", "code": "PT-03",
                                     "args": {"name": name, "lo": str(lo), "hi": str(hi)},
                                     "message": name + " 量程倒置"})
            except (TypeError, ValueError):
                findings.append({"level": "warn", "code": "PT-03B", "args": {"name": name},
                                 "message": name + " 量程非数值"})
        if not str(p.get("unit", "")).strip():
            findings.append({"level": "warn", "code": "PT-05", "args": {"name": name},
                             "message": name + " 缺少工程单位"})

    for i in range(len(spans)):
        for j in range(i + 1, len(spans)):
            a, b = spans[i], spans[j]
            if a[0] == b[0] and not (a[2] < b[1] or b[2] < a[1]):
                kind = "PT-01" if a[1] == b[1] else "PT-01"
                findings.append({"level": "error", "code": kind,
                                 "args": {"fc": str(a[0]), "name_a": a[3], "range_a": str(a[1]) + "-" + str(a[2]),
                                          "name_b": b[3], "range_b": str(b[1]) + "-" + str(b[2])},
                                 "message": "地址重叠"})

    errors = [f for f in findings if f["level"] == "error"]
    warns = [f for f in findings if f["level"] == "warn"]
    verdict = "不通过" if errors else ("有告警" if warns else "通过")
    if not findings:
        findings.append({"level": "info", "code": "PT-OK", "args": {"count": str(len(points))},
                         "message": "未发现一致性问题"})
    if source == "llm-extract":
        findings.insert(0, {"level": "info", "code": "PT-00",
                            "args": {"count": str(len(points))},
                            "message": "点位由模型抽取"})

    out = {
        "subject": str(payload.get("device", "")) + " 通信点表核对",
        "subject_args": {"device": str(payload.get("device", ""))},
        "verdict": verdict,
        "conclusion": "核对判定「" + verdict + "」：点位 " + str(len(points)) + " 条，错误 " + str(len(errors))
                      + " 项、告警 " + str(len(warns)) + " 项",
        "conclusion_args": {"verdict": verdict, "points": len(points),
                            "errors": len(errors), "warns": len(warns)},
        "findings": findings,
        "point_source": source,
        "metrics": {"point_count": len(points), "errors": len(errors), "warnings": len(warns),
                    "register_span": sum(s[2] - s[1] + 1 for s in spans)},
        "confidence": {"level": "medium" if source == "llm-extract" else "high",
                       "basis": ("点位由模型抽取，存在识别误差；一致性检查为确定性规则"
                                 if source == "llm-extract" else "点位为结构化输入，检查为确定性规则"),
                       "basis_code": "BASIS-LLM" if source == "llm-extract" else "BASIS"},
        "risks": [
            "未校验字节序（大端/小端）与上位机实际解析设置是否一致",
            "未校验从站地址与波特率等通信参数",
            "地址重叠判断按连续寄存器区间，未考虑厂商自定义映射",
        ],
        "risks_i18n": [
            {"code": "01", "text": "未校验字节序与上位机实际解析设置"},
            {"code": "02", "text": "未校验从站地址与波特率等通信参数"},
            {"code": "03", "text": "地址重叠判断未考虑厂商自定义映射"},
        ],
    }
    sys.stdout.write(json.dumps({"output": out}, ensure_ascii=True, sort_keys=True))
    return 0


if __name__ == "__main__":
    sys.exit(main())
