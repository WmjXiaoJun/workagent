#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""知识库摄入审计核验（哈希链 + 状态一致性）。

规则来源：真实 audit.jsonl 的字段语义。字段与事件类型取自生产数据，
示例行是合成的 —— 真实审计日志含你的文档路径与内容哈希，不入公开仓库。

  AC-01  同一 doc_id 的哈希链：old_sha256 必须等于该文档上一次的 new_sha256
  AC-02  source_changed=true 时 new_sha256 必须发生变化
  AC-03  semantic_stale=true 时 semantic_status 必须标记为需更新
  AC-04  line_count 相对该文档上一次下降超过阈值（默认 50%）——疑似截断
  AC-05  同一文档的事件时间戳不得倒退
  AC-06  非 kb.ingest 事件引用了未被任何 kb.ingest 建立过的 doc_id

纯确定性：同一份审计日志必然得到同样的核验结论（L3）。
"""
import json, sys

STALE_STATUS = ("stale", "need_update", "pending", "invalid")


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

    events = data.get("events")
    if not isinstance(events, list) or not events:
        fail("events 为空")
    if len(events) > 100000:
        fail("事件数超过 100000，超出 boundary.json 声明的规模")

    drop_ratio = float(data.get("line_drop_ratio", 0.5))
    if not (0 < drop_ratio < 1):
        fail("line_drop_ratio 必须在 (0,1) 区间")

    # 保持"按文档分组、组内保留日志原始顺序"——审计日志是追加写的有序流。
    # 若按 ts 排序，「时间戳倒退」这个要检测的现象会被排序抹掉。
    evs = [e for e in events if isinstance(e, dict)]
    evs = sorted(enumerate(evs), key=lambda p: (str(p[1].get("doc_id", "")), p[0]))
    evs = [e for _, e in evs]

    findings = []
    last_hash = {}
    last_lines = {}
    last_ts = {}
    known_docs = set()

    # upload 与 kb.ingest 都会建立文档；引用其它文档的事件才算孤儿
    for e in evs:
        if str(e.get("event_type", "")) in ("kb.ingest", "upload") and e.get("doc_id"):
            known_docs.add(str(e["doc_id"]))

    for e in evs:
        et = str(e.get("event_type", ""))
        did = str(e.get("doc_id") or "")
        eid = str(e.get("id", ""))
        ts = float(e.get("ts", 0))

        # AC-05 时间戳单调
        if did and did in last_ts and ts < last_ts[did]:
            findings.append({"level": "warn", "code": "AC-05",
                             "message": "时间戳倒退：doc=" + did + " id=" + eid
                                        + " 上一条 ts=%.3f 本条 ts=%.3f" % (last_ts[did], ts)})
        if did:
            last_ts[did] = max(ts, last_ts.get(did, ts))

        # AC-06 未知文档引用
        if did and did not in known_docs and et != "kb.ingest":
            findings.append({"level": "warn", "code": "AC-06",
                             "message": "引用了未经摄入建立的文档：doc=" + did + " 事件=" + et + " id=" + eid})

        if et != "kb.ingest":
            # 非摄入事件同样携带 new_sha256 / line_count 时更新基线，供后续比对
            nw = e.get("new_sha256")
            if nw and did:
                last_hash.setdefault(did, nw)
            lc0 = e.get("line_count")
            if isinstance(lc0, (int, float)) and did:
                last_lines.setdefault(did, lc0)
            continue

        old = e.get("old_sha256")
        new = e.get("new_sha256")

        # AC-01 哈希链
        if did in last_hash:
            if old != last_hash[did]:
                findings.append({"level": "error", "code": "AC-01",
                                 "message": "哈希链断裂：doc=" + did + " id=" + eid
                                            + " old_sha256=" + str(old)[:12] + "… 应为上一条 new_sha256="
                                            + str(last_hash[did])[:12] + "…"})
        elif old is not None:
            findings.append({"level": "info", "code": "AC-01",
                             "message": "首次摄入却带 old_sha256：doc=" + did + " id=" + eid})

        # AC-02 source_changed 语义
        if e.get("source_changed") is True:
            if did in last_hash and new == last_hash[did]:
                findings.append({"level": "error", "code": "AC-02",
                                 "message": "标称源已变更但哈希未变：doc=" + did + " id=" + eid + " sha=" + str(new)[:12] + "…"})

        # AC-03 semantic_stale 与 status 一致
        if e.get("semantic_stale") is True:
            st = str(e.get("semantic_status") or "").lower()
            if st not in STALE_STATUS:
                findings.append({"level": "warn", "code": "AC-03",
                                 "message": "标记语义过期但状态未更新：doc=" + did + " id=" + eid
                                            + " semantic_status=" + repr(e.get("semantic_status"))})

        # AC-04 行数突降
        lc = e.get("line_count")
        if isinstance(lc, (int, float)) and did in last_lines and last_lines[did] > 0:
            prev = last_lines[did]
            if lc < prev * (1 - drop_ratio):
                findings.append({"level": "error", "code": "AC-04",
                                 "message": "行数异常下降：doc=" + did + " id=" + eid
                                            + " 上次 " + str(int(prev)) + " 行 → 本次 " + str(int(lc)) + " 行（降幅 %.0f%%）"
                                            % ((1 - lc / prev) * 100)})
        if isinstance(lc, (int, float)):
            last_lines[did] = lc

        if new:
            last_hash[did] = new

    errors = [f for f in findings if f["level"] == "error"]
    warns = [f for f in findings if f["level"] == "warn"]
    verdict = "不一致" if errors else ("有告警" if warns else "一致")
    info = [f for f in findings if f["level"] == "info"]
    if not errors and not warns:
        findings = [{"level": "info", "code": "AC-OK",
                     "message": "事件 %d 条、文档 %d 个，六项核验全部通过" % (len(evs), len(known_docs))}]
    else:
        findings = errors + warns + info

    out = {
        "subject": str(data.get("system", "知识库审计日志")) + " 一致性核验",
        "verdict": verdict,
        "conclusion": "核验判定「" + verdict + "」：事件 %d 条 / 文档 %d 个，错误 %d 项、告警 %d 项"
                      % (len(evs), len(known_docs), len(errors), len(warns)),
        "findings": findings,
        "metrics": {"events": len(evs), "documents": len(known_docs),
                    "errors": len(errors), "warnings": len(warns),
                    "chain_breaks": len([f for f in errors if f["code"] == "AC-01"])},
        "confidence": {"level": "high",
                       "basis": "六项校验全部基于事件字段的确定性语义，不依赖模型；字段定义取自真实审计日志"},
        "risks": [
            "只校验事件序列内部自洽，不校验文档内容是否正确",
            "哈希链按 doc_id 分组，若同一文档跨多个来源路径需先统一 doc_id 口径",
            "line_count 突降也可能是合法的文档精简，需人工确认",
        ],
    }
    sys.stdout.write(json.dumps({"output": out}, ensure_ascii=True, sort_keys=True))
    return 0


if __name__ == "__main__":
    sys.exit(main())
