#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""嵌入式 · 引脚 / 中断 / DMA 资源冲突检测。

把硬件资源分配表当输入，输出**冲突清单**。冲突就是 bug，编译期就能验真。
纯确定性：同一张分配表必然得到同样的冲突列表（L3）。

检测项（示例规则集，可扩展）：
  PIN-01  同一引脚被分配给两个不同信号
  PIN-02  保留了调试口引脚却被占用
  PIN-03  引脚命名不符合 P<端口><序号> 规则
  IRQ-01  中断抢占优先级重复（本规则集要求唯一）
  IRQ-02  中断优先级超出 MCU 允许范围
  DMA-01  DMA 通道被重复分配
  DMA-02  DMA 通道被分配给多个外设
"""
import json, re, sys


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

    pins = data.get("pins") or []
    irqs = data.get("interrupts") or []
    dmas = data.get("dma") or []
    reserved = set(data.get("reserved_pins") or [])
    prio_max = int(data.get("priority_max", 15))
    if not isinstance(pins, list):
        fail("pins 不是数组")
    if len(pins) > 2000:
        fail("pins 超过 2000 条，超出 boundary.json 声明的规模")

    findings = []

    # PIN-01 / PIN-02 / PIN-03
    seen = {}
    for row in pins:
        row = row if isinstance(row, dict) else {}
        pin = str(row.get("pin", "")).strip()
        sig = str(row.get("signal", "")).strip()
        if not pin:
            findings.append({"level": "error", "code": "PIN-00", "args": {},
                             "message": "存在未填引脚名的分配行"})
            continue
        if not re.fullmatch(r"P[A-Z][0-9]{1,2}", pin):
            findings.append({"level": "error", "code": "PIN-03", "args": {"pin": pin},
                             "message": "引脚命名不合法：" + pin})
        if pin in reserved:
            findings.append({"level": "error", "code": "PIN-02", "args": {"pin": pin, "signal": sig},
                             "message": "保留了调试口引脚 " + pin})
        if pin in seen and seen[pin] != sig:
            findings.append({"level": "error", "code": "PIN-01",
                             "args": {"pin": pin, "a": seen[pin], "b": sig},
                             "message": "引脚 " + pin + " 被重复分配"})
        else:
            seen[pin] = sig

    # IRQ-01 / IRQ-02
    prio_map = {}
    for row in irqs:
        row = row if isinstance(row, dict) else {}
        irq = str(row.get("irq", "")).strip()
        pr = row.get("priority")
        if isinstance(pr, bool) or not isinstance(pr, int):
            findings.append({"level": "error", "code": "IRQ-02", "args": {"irq": irq},
                             "message": "中断 " + irq + " 优先级缺失"})
            continue
        if pr < 0 or pr > prio_max:
            findings.append({"level": "error", "code": "IRQ-02B",
                             "args": {"irq": irq, "priority": str(pr), "max": str(prio_max)},
                             "message": "中断 " + irq + " 优先级越界"})
        prio_map.setdefault(pr, []).append(irq)
    for pr, names in sorted(prio_map.items()):
        if len(names) > 1:
            findings.append({"level": "error", "code": "IRQ-01",
                             "args": {"priority": str(pr), "count": str(len(names)), "names": ", ".join(names)},
                             "message": "抢占优先级 " + str(pr) + " 被共用"})

    # DMA-01 / DMA-02
    chan_map = {}
    for row in dmas:
        row = row if isinstance(row, dict) else {}
        ch = str(row.get("channel", "")).strip()
        per = str(row.get("peripheral", "")).strip()
        chan_map.setdefault(ch, []).append(per)
    for ch, pers in sorted(chan_map.items()):
        uniq = sorted(set(pers))
        if len(pers) > 1 and len(uniq) > 1:
            findings.append({"level": "error", "code": "DMA-02",
                             "args": {"channel": ch, "peripherals": ", ".join(uniq)},
                             "message": "DMA 通道 " + ch + " 分配给多个外设"})
        elif len(pers) > 1:
            findings.append({"level": "warn", "code": "DMA-01",
                             "args": {"channel": ch, "count": str(len(pers))},
                             "message": "DMA 通道 " + ch + " 被重复声明"})

    errors = [f for f in findings if f["level"] == "error"]
    warns = [f for f in findings if f["level"] == "warn"]
    verdict = "存在冲突" if errors else ("有告警" if warns else "无冲突")
    if not findings:
        findings.append({"level": "info", "code": "RES-OK",
                         "args": {"pins": str(len(pins)), "irqs": str(len(irqs)), "dmas": str(len(dmas))},
                         "message": "未发现资源冲突"})

    out = {
        "subject": str(data.get("mcu", "")) + " 硬件资源冲突检测",
        "verdict": verdict,
        # 原结论里嵌了"首批：<第一条 finding 的文案>" —— 那等于在句子里塞执行体的人话，
        # 且语言相关。去掉；首条发现在 findings 里本来就有。
        "conclusion": "判定「" + verdict + "」：错误 " + str(len(errors)) + " 项，告警 " + str(len(warns)) + " 项",
        "conclusion_args": {"verdict": verdict, "errors": len(errors), "warns": len(warns)},
        "findings": findings,
        "metrics": {"pins": len(pins), "interrupts": len(irqs), "dma": len(dmas),
                    "errors": len(errors), "warnings": len(warns)},
        "confidence": {"level": "high",
                       "basis": "规则集为确定性检查，无模型参与；同一分配表必然得到同一清单"},
        "risks": [
            "规则集是示例，未覆盖复用功能表、时钟树与电气约束（需按具体 MCU 数据手册扩展）",
            "引脚冲突不等于功能不可实现：部分 MCU 允许分时复用，需人工确认",
        ],
    }
    sys.stdout.write(json.dumps({"output": out}, ensure_ascii=True, sort_keys=True))
    return 0


if __name__ == "__main__":
    sys.exit(main())
