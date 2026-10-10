#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""上位机 · 通信负载与轮询周期测算（Modbus RTU / TCP）。

给定链路参数与点表，算出**一轮轮询要多久**、能否满足目标周期、瓶颈在哪、怎么优化。
纯确定性：同一份输入必然得到同样的结论（L3）。

规则来源：Modbus 协议帧格式（公开规范）
  RTU 帧 = 从站地址(1) + 功能码(1) + 数据(N) + CRC(2)
  FC03/04 请求 = 8 字节（地址+功能码+起始地址(2)+数量(2)+CRC(2)）
  FC03/04 响应 = 5 + 2×寄存器数 字节
  字符位数 = 1 起始位 + 数据位 + 校验位 + 停止位
  帧间静默 = 3.5 个字符时间（RTU 规范要求）
  单次请求最多 125 个寄存器（Modbus 规范上限）

关键优化：**相邻寄存器的点可以合并成一次请求**。合并与否，轮询周期差好几倍。
"""
import json, sys

MAX_REGS_PER_REQ = 125


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

    link = data.get("link") or {}
    mode = str(link.get("mode", "rtu")).lower()
    if mode not in ("rtu", "tcp"):
        fail("mode 必须是 rtu 或 tcp")
    baud = link.get("baud", 9600)
    data_bits = int(link.get("data_bits", 8))
    parity = str(link.get("parity", "N")).upper()
    stop_bits = int(link.get("stop_bits", 1))
    turnaround_ms = float(link.get("turnaround_ms", 15.0))
    if parity not in ("N", "E", "O"):
        fail("parity 必须是 N / E / O")
    if data_bits not in (7, 8):
        fail("data_bits 必须是 7 或 8")
    if stop_bits not in (1, 2):
        fail("stop_bits 必须是 1 或 2")
    if mode == "rtu" and (not isinstance(baud, (int, float)) or baud <= 0):
        fail("baud 必须为正数")

    target_ms = float(data.get("target_cycle_ms", 1000))
    if target_ms <= 0:
        fail("target_cycle_ms 必须为正数")

    points = data.get("points")
    if not isinstance(points, list) or not points:
        fail("points 为空")
    if len(points) > 1000:
        fail("points 超过 1000，超出 boundary.json 声明的规模")

    # ---- 字符时间与帧时间 ----
    if mode == "rtu":
        bits_per_char = 1 + data_bits + (0 if parity == "N" else 1) + stop_bits
        char_ms = bits_per_char / float(baud) * 1000.0
        silence_ms = 3.5 * char_ms
        # 以太网按 100 Mbit/s 估算帧开销（含以太网/IP/TCP/MBAP 头约 66 字节）
        char_ms = 0.0
        silence_ms = 0.0
        eth_overhead_bytes = 66
        tcp_request_ms = (eth_overhead_bytes + 8) * 8 / 100000.0
        tcp_response_base_ms = (eth_overhead_bytes + 5) * 8 / 100000.0
        tcp_per_reg_ms = 2 * 8 / 100000.0
    else:
        bits_per_char = 0
        char_ms = 0.0
        silence_ms = 0.0

    def frame_ms(req_bytes, resp_bytes):
        if mode == "rtu":
            return (req_bytes + resp_bytes) * char_ms + 2 * silence_ms
        return tcp_request_ms + tcp_response_base_ms + (resp_bytes - 5) * tcp_per_reg_ms

    # ---- 解析点位，做 Modbus 参考地址归一化 ----
    norm = []
    for p in points:
        p = p if isinstance(p, dict) else {}
        name = str(p.get("name", "")).strip()
        fc = p.get("function_code")
        addr = p.get("address")
        regs = p.get("registers", 1)
        if fc not in (1, 2, 3, 4):
            findings_fc = True
            fail("点位「" + name + "」功能码 " + str(fc) + " 不支持（本测算只覆盖 1/2/3/4）")
        try:
            addr = int(addr)
            regs = int(regs)
        except (TypeError, ValueError):
            fail("点位「" + name + "」地址或寄存器数非法")
        if regs < 1 or regs > MAX_REGS_PER_REQ:
            fail("点位「" + name + "」寄存器数 " + str(regs) + " 超出 1~125")
        # 参考地址归一化：4xxxxx=保持寄存器、3xxxxx=输入寄存器、1xxxxx=离散输入、0xxxxx=线圈
        if 40001 <= addr <= 49999:
            addr -= 40001
        elif 30001 <= addr <= 39999:
            addr -= 30001
        elif 10001 <= addr <= 19999:
            addr -= 10001
        elif 1 <= addr <= 9999:
            addr -= 1
        norm.append({"name": name, "fc": int(fc), "addr": addr, "regs": regs,
                     "end": addr + regs - 1})

    # ---- 独立请求（不合并）----
    solo = []
    for n in sorted(norm, key=lambda x: (x["fc"], x["addr"])):
        req_time = frame_ms(8, 5 + 2 * n["regs"]) + turnaround_ms
        solo.append({"name": n["name"], "requests": 1, "cost_ms": round(req_time, 3)})
    solo_total = sum(x["cost_ms"] for x in solo)

    # ---- 合并连续寄存器（按功能码分组，贪心合并）----
    merged = []
    for fc in (3, 4, 1, 2):
        group = sorted([n for n in norm if n["fc"] == fc], key=lambda x: x["addr"])
        i = 0
        while i < len(group):
            start = group[i]["addr"]
            end = group[i]["end"]
            names = [group[i]["name"]]
            j = i + 1
            while j < len(group):
                nxt = group[j]
                new_end = max(end, nxt["end"])
                if nxt["addr"] <= end + 1 and (new_end - start + 1) <= MAX_REGS_PER_REQ:
                    end = new_end
                    names.append(nxt["name"])
                    j += 1
                else:
                    break
            regs = end - start + 1
            merged.append({"fc": fc, "start": start, "regs": regs, "names": names,
                           "cost_ms": round(frame_ms(8, 5 + 2 * regs) + turnaround_ms, 3)})
            i = j
    merged_total = sum(x["cost_ms"] for x in merged)

    findings = []
    if merged_total <= target_ms:
        verdict = "满足"
    else:
        verdict = "不满足"
    findings.append({"level": "info", "code": "POLL-MERGED",
                     "args": {"requests": len(merged), "cycle_ms": "%.1f" % merged_total,
                              "pct": "%.0f" % (merged_total / target_ms * 100)},
                     "message": "合并后一轮耗时 %d 次请求" % len(merged)})
    findings.append({"level": "info", "code": "POLL-SOLO",
                     "args": {"requests": len(solo), "cycle_ms": "%.1f" % solo_total,
                              "saved_pct": "%.1f" % ((solo_total - merged_total) / solo_total * 100 if solo_total else 0)},
                     "message": "不合并一轮耗时 %d 次请求" % len(solo)})
    if verdict == "不满足":
        findings.append({"level": "error", "code": "POLL-OVER",
                         "args": {"cycle_ms": "%.1f" % merged_total, "target_ms": "%.0f" % target_ms,
                                  "margin_ms": "%.1f" % (target_ms - merged_total),
                                  "short_ms": "%.1f" % (merged_total - target_ms)},
                         "message": "一轮耗时超过目标周期"})
        if mode == "rtu" and baud < 115200:
            findings.append({"level": "warn", "code": "POLL-SUGGEST",
                             "args": {"baud": baud, "cycle_ms": "%.1f" % (merged_total * baud / 115200.0)},
                             "message": "建议提高波特率"})
        findings.append({"level": "warn", "code": "POLL-SUGGEST-B",
                         "args": {"target_ms": "%.0f" % (merged_total * 1.2)},
                         "message": "建议放宽目标周期"})
    else:
        findings.append({"level": "info", "code": "POLL-ROOM",
                         "args": {"margin_ms": "%.1f" % (target_ms - merged_total),
                                  "pct": "%.0f" % ((1 - merged_total / target_ms) * 100),
                                  "max_points": int(len(points) * target_ms / merged_total) if merged_total else len(points)},
                         "message": "仍有余量"})
    # 慢点识别
    slow = sorted(merged, key=lambda x: -x["cost_ms"])[:3]
    for s in slow:
        findings.append({"level": "info", "code": "POLL-SLOW",
                         "args": {"fc": "%02d" % s["fc"], "start": s["start"], "regs": s["regs"],
                                  "names": ", ".join(s["names"][:3]), "cost_ms": "%.1f" % s["cost_ms"]},
                         "message": "最慢请求 FC%02d" % s["fc"]})

    out = {
        "subject": str(link.get("name", "")) + " 轮询周期测算",
        "verdict": verdict,
        "conclusion": "一轮轮询 %.1f ms（合并后 %d 次请求），目标周期 %.0f ms —— 判定「%s」"
                      % (merged_total, len(merged), target_ms, verdict),
        "conclusion_args": {"cycle_ms": "%.1f" % merged_total, "requests": len(merged),
                            "target_ms": "%.0f" % target_ms, "verdict": verdict},
        "findings": findings,
        "metrics": {
            "mode": mode,
            "char_ms": round(char_ms, 4),
            "silence_ms": round(silence_ms, 4),
            "turnaround_ms": turnaround_ms,
            "point_count": len(points),
            "requests_solo": len(solo),
            "requests_merged": len(merged),
            "cycle_solo_ms": round(solo_total, 2),
            "cycle_merged_ms": round(merged_total, 2),
            "target_ms": target_ms,
            "margin_ms": round(target_ms - merged_total, 2),
            "utilization": round(merged_total / target_ms, 4),
        },
        "requests": merged,
        "confidence": {"level": "high",
                       "basis": "按 Modbus 帧格式与字符时间计算，纯确定性；不含从站内部处理耗时以外的随机因素"},
        "risks": [
            "turnaround_ms 为估计值，实际从站响应间隔需现场实测（不同厂商差异可达数倍）",
            "未计入网络交换机时延、无线链路重传与丢包重试",
            "未计入主站软件自身的调度与处理开销",
        ],
    }
    sys.stdout.write(json.dumps({"output": out}, ensure_ascii=True, sort_keys=True))
    return 0


if __name__ == "__main__":
    sys.exit(main())
