#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""嵌入式 · 固件资源预算测算（Flash / RAM / 任务栈）。

把各模块占用汇总，与 MCU 容量比对，并检查任务栈余量。
纯确定性：同一份预算表必然得到同样的结论（L3）。

判定规则（阈值可在输入中覆盖）：
  FR-01  Flash 占用 > 容量                     error
  FR-02  Flash 占用率 > 告警阈值（默认 85%）    warn
  FR-03  RAM 占用 > 容量                       error
  FR-04  RAM 占用率 > 告警阈值（默认 80%）      warn
  FR-05  任务栈余量 < 下限（默认 20%）          error —— 栈溢出是现场最难查的故障
  FR-06  栈峰值超过分配量（说明已经溢出）        error
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

    flash_cap = num(data.get("flash_capacity_kb"), "flash_capacity_kb")
    ram_cap = num(data.get("ram_capacity_kb"), "ram_capacity_kb")
    if flash_cap <= 0 or ram_cap <= 0:
        fail("容量必须为正数")

    th = data.get("thresholds") or {}
    flash_warn = float(th.get("flash_warn", 0.85))
    ram_warn = float(th.get("ram_warn", 0.80))
    stack_min = float(th.get("stack_min_margin", 0.20))

    flash_items = data.get("flash") or []
    ram_items = data.get("ram") or []
    stacks = data.get("stacks") or []
    if not isinstance(flash_items, list) or not flash_items:
        fail("flash 模块清单为空")
    if len(flash_items) > 200 or len(ram_items) > 200 or len(stacks) > 100:
        fail("清单条数超出 boundary.json 声明的规模")

    flash_used = 0.0
    flash_detail = []
    for it in flash_items:
        it = it if isinstance(it, dict) else {}
        kb = num(it.get("kb"), "flash.kb")
        if kb < 0:
            fail("flash 占用为负")
        flash_used += kb
        flash_detail.append({"module": str(it.get("module", "")), "kb": round(kb, 2)})
    flash_detail.sort(key=lambda x: -x["kb"])

    ram_static = 0.0
    ram_detail = []
    for it in ram_items:
        it = it if isinstance(it, dict) else {}
        kb = num(it.get("kb"), "ram.kb")
        if kb < 0:
            fail("ram 占用为负")
        ram_static += kb
        ram_detail.append({"module": str(it.get("module", "")), "kb": round(kb, 2)})
    ram_detail.sort(key=lambda x: -x["kb"])

    heap_kb = float(data.get("heap_kb", 0))
    stack_total_b = 0.0
    stack_rows = []
    for st in stacks:
        st = st if isinstance(st, dict) else {}
        alloc = num(st.get("allocated_b"), "stacks.allocated_b")
        peak = num(st.get("peak_b"), "stacks.peak_b")
        if alloc <= 0:
            fail("栈分配量必须为正数")
        margin = (alloc - peak) / alloc
        stack_total_b += alloc
        stack_rows.append({"task": str(st.get("task", "")), "allocated_b": int(alloc),
                           "peak_b": int(peak), "margin": round(margin, 4)})
    stack_rows.sort(key=lambda x: x["margin"])
    stack_kb = stack_total_b / 1024.0

    ram_used = ram_static + stack_kb + heap_kb
    flash_pct = flash_used / flash_cap
    ram_pct = ram_used / ram_cap

    findings = []
    if flash_used > flash_cap:
        findings.append({"level": "error", "code": "FR-01",
                         "args": {"used": "%.1f" % flash_used, "cap": "%.1f" % flash_cap,
                                  "over": "%.1f" % (flash_used - flash_cap)},
                         "message": "Flash 超容量"})
    elif flash_pct > flash_warn:
        findings.append({"level": "warn", "code": "FR-02",
                         "args": {"pct": "%.1f" % (flash_pct * 100), "threshold": "%.0f" % (flash_warn * 100),
                                  "margin": "%.1f" % (flash_cap - flash_used)},
                         "message": "Flash 占用率超阈值"})
    else:
        findings.append({"level": "info", "code": "FR-02-OK",
                         "args": {"used": "%.1f" % flash_used, "cap": "%.1f" % flash_cap,
                                  "pct": "%.1f" % (flash_pct * 100)},
                         "message": "Flash 余量充足"})

    if ram_used > ram_cap:
        findings.append({"level": "error", "code": "FR-03",
                         "args": {"used": "%.1f" % ram_used, "cap": "%.1f" % ram_cap,
                                  "over": "%.1f" % (ram_used - ram_cap)},
                         "message": "RAM 超容量"})
    elif ram_pct > ram_warn:
        findings.append({"level": "warn", "code": "FR-04",
                         "args": {"pct": "%.1f" % (ram_pct * 100), "threshold": "%.0f" % (ram_warn * 100),
                                  "margin": "%.1f" % (ram_cap - ram_used)},
                         "message": "RAM 占用率超阈值"})
    else:
        findings.append({"level": "info", "code": "FR-04-OK",
                         "args": {"used": "%.1f" % ram_used, "cap": "%.1f" % ram_cap,
                                  "pct": "%.1f" % (ram_pct * 100)},
                         "message": "RAM 余量充足"})

    for r in stack_rows:
        if r["peak_b"] > r["allocated_b"]:
            findings.append({"level": "error", "code": "FR-06",
                             "args": {"task": r["task"], "peak": r["peak_b"], "allocated": r["allocated_b"]},
                             "message": "任务「%s」栈溢出" % r["task"]})
        elif r["margin"] < stack_min:
            findings.append({"level": "error", "code": "FR-05",
                             "args": {"task": r["task"], "margin_pct": "%.0f" % (r["margin"] * 100),
                                      "peak": r["peak_b"], "allocated": r["allocated_b"],
                                      "floor": "%.0f" % (stack_min * 100)},
                             "message": "任务「%s」栈余量不足" % r["task"]})

    errors = [f for f in findings if f["level"] == "error"]
    warns = [f for f in findings if f["level"] == "warn"]
    verdict = "超限" if errors else ("告警" if warns else "通过")

    out = {
        "subject": str(data.get("mcu", "")) + " 固件资源预算",
        "verdict": verdict,
        "conclusion": "判定「%s」：Flash %.1f/%.1f KB（%.1f%%），RAM %.1f/%.1f KB（%.1f%%）；错误 %d 项、告警 %d 项"
                      % (verdict, flash_used, flash_cap, flash_pct * 100,
                         ram_used, ram_cap, ram_pct * 100, len(errors), len(warns)),
        "conclusion_args": {"verdict": verdict,
                            "flash_used": "%.1f" % flash_used, "flash_cap": "%.1f" % flash_cap,
                            "flash_pct": "%.1f" % (flash_pct * 100),
                            "ram_used": "%.1f" % ram_used, "ram_cap": "%.1f" % ram_cap,
                            "ram_pct": "%.1f" % (ram_pct * 100),
                            "errors": len(errors), "warns": len(warns)},
        "findings": findings,
        "metrics": {
            "flash_used_kb": round(flash_used, 2), "flash_capacity_kb": flash_cap,
            "flash_ratio": round(flash_pct, 4),
            "ram_used_kb": round(ram_used, 2), "ram_capacity_kb": ram_cap,
            "ram_ratio": round(ram_pct, 4),
            "ram_static_kb": round(ram_static, 2),
            "stack_total_kb": round(stack_kb, 2),
            "heap_kb": round(heap_kb, 2),
            "errors": len(errors), "warnings": len(warns),
        },
        "flash_top": flash_detail[:5],
        "ram_top": ram_detail[:5],
        "stack_rows": stack_rows,
        "confidence": {"level": "high",
                       "basis": "汇总与容量比对为纯算术；阈值来自实践惯例且可在输入中覆盖"},
        "risks": [
            "栈峰值为实测高水位，未覆盖尚未走到的分支；建议留足余量",
            "未计入中断栈、DMA 缓冲区与运行时动态分配",
            "Flash 未计入未来 OTA 双分区与升级余量",
        ],
    }
    sys.stdout.write(json.dumps({"output": out}, ensure_ascii=True, sort_keys=True))
    return 0


if __name__ == "__main__":
    sys.exit(main())
