#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""制造 · SPC 控制图判异（Nelson / 西方电气 8 条准则）。

输入一串按时间顺序的测量值，判定过程是否"失控"，并指出**具体在哪一点违反了哪条准则**。
纯确定性：同一串数据必然得到同样的判定（L3）。

准则来源：Western Electric Rules / Nelson Rules（标准 SPC 判异规则，公开文献一致）。
中心线与西格玛：优先使用输入给定值；未给定时用单值-移动极差(I-MR)估计（MRbar/1.128）。
"""
import json, sys


def fail(reason):
    sys.stderr.write("BOUNDARY_VIOLATION: " + reason + "\n")
    sys.exit(2)


def main():
    if len(sys.argv) < 2:
        fail("缺少输入文件参数")
    with open(sys.argv[1], "r", encoding="utf-8") as fh:
        data = json.load(fh)
    if isinstance(data.get("input"), dict):
        data = data["input"]

    values = data.get("values")
    if not isinstance(values, list) or len(values) < 8:
        fail("values 少于 8 个点，无法做判异（至少需要 8 点）")
    if len(values) > 5000:
        fail("values 超过 5000 点，超出 boundary.json 声明的规模")
    try:
        xs = [float(v) for v in values]
    except (TypeError, ValueError):
        fail("values 含非数字")

    cl = data.get("center_line")
    sigma = data.get("sigma")
    if cl is None:
        cl = sum(xs) / len(xs)
    cl = float(cl)
    if sigma is None:
        mr = [abs(xs[i] - xs[i - 1]) for i in range(1, len(xs))]
        mrbar = sum(mr) / len(mr) if mr else 0.0
        sigma = mrbar / 1.128
    sigma = float(sigma)
    if sigma <= 0:
        fail("西格玛为 0 或负，数据无波动，判异无意义")

    def zone(x):
        d = (x - cl) / sigma
        if d > 3: return 3
        if d > 2: return 2
        if d > 1: return 1
        if d < -3: return -3
        if d < -2: return -2
        if d < -1: return -1
        return 0

    z = [zone(x) for x in xs]
    violations = []

    def add(rule, idx, desc):
        violations.append({"rule": rule, "index": idx + 1, "value": xs[idx], "desc": desc})

    # R1 一点超出 3σ
    for i, zz in enumerate(z):
        if abs(zz) == 3:
            add("R1", i, "单点超出 3σ")
    # R2 连续 3 点中有 2 点超出同侧 2σ
    for i in range(2, len(z)):
        w = z[i - 2:i + 1]
        if sum(1 for v in w if v >= 2) >= 2 or sum(1 for v in w if v <= -2) >= 2:
            add("R2", i, "连续 3 点中 2 点超出同侧 2σ")
    # R3 连续 5 点中有 4 点超出同侧 1σ
    for i in range(4, len(z)):
        w = z[i - 4:i + 1]
        if sum(1 for v in w if v >= 1) >= 4 or sum(1 for v in w if v <= -1) >= 4:
            add("R3", i, "连续 5 点中 4 点超出同侧 1σ")
    # R4 连续 8 点在中心线同侧
    for i in range(7, len(z)):
        w = z[i - 7:i + 1]
        if all(v > 0 for v in w) or all(v < 0 for v in w):
            add("R4", i, "连续 8 点在中心线同侧")
    # R5 连续 6 点单调递增或递减
    for i in range(5, len(z)):
        w = xs[i - 5:i + 1]
        if all(w[k] < w[k + 1] for k in range(5)) or all(w[k] > w[k + 1] for k in range(5)):
            add("R5", i, "连续 6 点单调趋势")
    # R6 连续 15 点落在 1σ 内（分层，波动异常小）
    for i in range(14, len(z)):
        if all(z[k] == 0 for k in range(i - 14, i + 1)):
            add("R6", i, "连续 15 点落在 1σ 内（分层）")
    # R7 连续 14 点交替上下
    for i in range(13, len(z)):
        w = xs[i - 13:i + 1]
        alt = all((w[k] - w[k + 1]) * (w[k + 1] - w[k + 2]) < 0 for k in range(12))
        if alt:
            add("R7", i, "连续 14 点交替上下")
    # R8 连续 8 点超出 1σ（任意一侧）
    for i in range(7, len(z)):
        if all(abs(z[k]) >= 1 for k in range(i - 7, i + 1)):
            add("R8", i, "连续 8 点超出 1σ")

    errors = [v for v in violations if v["rule"] in ("R1", "R2", "R3", "R4")]
    verdict = "失控" if errors else ("预警" if violations else "受控")

    findings = [
        {"level": "error" if v["rule"] in ("R1", "R2", "R3", "R4") else "warn",
         "code": v["rule"], "message": "第 " + str(v["index"]) + " 点：" + v["desc"]}
        for v in violations[:40]
    ]
    if not findings:
        findings.append({"level": "info", "code": "SPC-OK",
                         "args": {"rules": 8},
                         "message": "8 条判异准则均未触发，过程受控"})

    # 两种结论形态由**事实**决定，文案仍来自 messages.json。
    if violations:
        concl = "过程判定「" + verdict + "」：共触发 " + str(len(violations)) + " 处判异（" \
                + ", ".join(sorted(set(v["rule"] for v in violations))) + "）"
        concl_code = "CONCLUSION"
    else:
        concl = "过程判定「受控」：8 条判异准则均未触发"
        concl_code = "CONCLUSION_CLEAN"

    out = {
        "subject": str(data.get("process", "")) + " 控制图判异",
        "verdict": verdict,
        "conclusion": concl,
        "conclusion_code": concl_code,
        "conclusion_args": {"verdict": verdict, "count": len(violations),
                            "rules": ", ".join(sorted(set(v["rule"] for v in violations))) or "-"},
        "findings": findings,
        "violations": violations,
        "metrics": {"n": len(xs), "center_line": round(cl, 6), "sigma": round(sigma, 6),
                    "ucl": round(cl + 3 * sigma, 6), "lcl": round(cl - 3 * sigma, 6),
                    "violation_count": len(violations)},
        "confidence": {"level": "high",
                       "basis": "判异准则为公开标准；中心线与西格玛由输入或 I-MR 法确定，纯确定性"},
        "risks": [
            "控制限基于历史数据，过程均值漂移后需重新计算基线",
            "R6（分层）与 R7（交替）属于异常好/周期性信号，需结合工艺判断",
            "样本量 " + str(len(xs)) + " 偏少时 R6/R7 不会触发，属正常",
        ],
    }
    sys.stdout.write(json.dumps({"output": out}, ensure_ascii=True, sort_keys=True))
    return 0


if __name__ == "__main__":
    sys.exit(main())
