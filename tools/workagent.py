#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
WorkAgent CLI —— 零依赖的智能体包工具链。

命令:
    workagent check    校验包结构与兼容声明
    workagent adapt    从 agent.yaml 生成适配层 (Agent Plugins / A2A)
    workagent attest   计算 Attestation Closure 摘要
    workagent run      执行并产出证据包
    workagent verify   复现校验：同输入是否产出同结论
    workagent seal     生成全包摘要 (signature/agent.sig)

设计原则:
    1. 零第三方依赖，仅标准库。
    2. 真源唯一：agent.yaml。adapters/ 是生成物。
    3. 诚实：没有私钥就只出摘要，不冒充"已签名"。
"""

import argparse
import hashlib
import json
import platform
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path

SPEC_VERSION = "0.3.0"
TOOL_VERSION = "0.3.0"
EVIDENCE_DIR = "out/evidence"

REQUIRED_FILES = [
    "agent.yaml",
    "compat.json",
    "inputs.schema.json",
    "artifacts/output_schema.json",
    "execution/workflow.yml",
    "execution/runtime.dockerfile",
    "provenance/trace.json",
    "provenance/data_sources.json",
    "provenance/assumptions.json",
    "attest/closure.json",
    "x-workagent/boundary.json",
    "x-workagent/test_suite/unit.json",
    "x-workagent/test_suite/regression.json",
    "x-workagent/test_suite/sensitivity.json",
    "x-workagent/test_suite/stress.json",
]


# --------------------------------------------------------------------------
# 极简 YAML 子集解析器（只覆盖本规范用到的语法，避免引入 PyYAML 依赖）
# --------------------------------------------------------------------------

def _strip_comment(line):
    out = []
    quote = None
    for ch in line:
        if quote:
            out.append(ch)
            if ch == quote:
                quote = None
        elif ch in "\"'":
            quote = ch
            out.append(ch)
        elif ch == "#":
            break
        else:
            out.append(ch)
    return "".join(out)


def _split_top(text):
    parts, buf, depth, quote = [], [], 0, None
    for ch in text:
        if quote:
            buf.append(ch)
            if ch == quote:
                quote = None
        elif ch in "\"'":
            quote = ch
            buf.append(ch)
        elif ch in "[{":
            depth += 1
            buf.append(ch)
        elif ch in "]}":
            depth -= 1
            buf.append(ch)
        elif ch == "," and depth == 0:
            parts.append("".join(buf))
            buf = []
        else:
            buf.append(ch)
    if buf:
        parts.append("".join(buf))
    return [p.strip() for p in parts if p.strip()]


def _scalar(text):
    s = text.strip()
    if len(s) >= 2 and s[0] == s[-1] and s[0] in "\"'":
        return s[1:-1]
    if s.startswith("[") and s.endswith("]"):
        return [_scalar(x) for x in _split_top(s[1:-1])]
    if s.startswith("{") and s.endswith("}"):
        result = {}
        for pair in _split_top(s[1:-1]):
            k, _, v = pair.partition(":")
            result[k.strip()] = _scalar(v)
        return result
    low = s.lower()
    if low == "true":
        return True
    if low == "false":
        return False
    if low in ("null", "~", ""):
        return None
    try:
        return int(s)
    except ValueError:
        pass
    try:
        return float(s)
    except ValueError:
        return s


def parse_yaml(text):
    lines = []
    for raw in text.splitlines():
        line = _strip_comment(raw).rstrip()
        if not line.strip():
            continue
        indent = len(line) - len(line.lstrip(" "))
        lines.append((indent, line.strip()))
    pos = [0]

    def parse_block(indent):
        if pos[0] >= len(lines):
            return None
        if lines[pos[0]][0] == indent and lines[pos[0]][1].startswith("- "):
            items = []
            while pos[0] < len(lines) and lines[pos[0]][0] == indent and lines[pos[0]][1].startswith("- "):
                body = lines[pos[0]][1][2:].strip()
                pos[0] += 1
                if ":" in body and not body.startswith("{"):
                    key, _, val = body.partition(":")
                    node = {}
                    if val.strip():
                        node[key.strip()] = _scalar(val)
                    if pos[0] < len(lines) and lines[pos[0]][0] > indent:
                        child = parse_block(lines[pos[0]][0])
                        if key.strip() in node:
                            if isinstance(child, dict):
                                node.update(child)
                        else:
                            node[key.strip()] = child
                    items.append(node)
                else:
                    items.append(_scalar(body))
            return items
        mapping = {}
        while pos[0] < len(lines) and lines[pos[0]][0] == indent and not lines[pos[0]][1].startswith("- "):
            key, _, val = lines[pos[0]][1].partition(":")
            key = key.strip()
            val = val.strip()
            pos[0] += 1
            if val == "":
                if pos[0] < len(lines) and lines[pos[0]][0] > indent:
                    mapping[key] = parse_block(lines[pos[0]][0])
                else:
                    mapping[key] = None
            else:
                mapping[key] = _scalar(val)
        return mapping

    return parse_block(lines[0][0]) if lines else {}


# --------------------------------------------------------------------------
# 通用工具
# --------------------------------------------------------------------------

def sha256_bytes(data):
    return hashlib.sha256(data).hexdigest()


def sha256_file(path):
    return sha256_bytes(Path(path).read_bytes())


def canonical_json(obj):
    return json.dumps(obj, sort_keys=True, ensure_ascii=False, separators=(",", ":"))


def read_json(path):
    return json.loads(Path(path).read_text(encoding="utf-8"))


def write_json(path, obj):
    p = Path(path)
    p.parent.mkdir(parents=True, exist_ok=True)
    p.write_text(json.dumps(obj, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def now_iso():
    return datetime.now(timezone.utc).astimezone().isoformat(timespec="seconds")


def resolve_root(arg):
    return Path(arg).resolve()


def load_manifest(root):
    manifest_path = root / "agent.yaml"
    if not manifest_path.exists():
        raise SystemExit("找不到 agent.yaml —— 请确认这是 WorkAgent 包根目录")
    return parse_yaml(manifest_path.read_text(encoding="utf-8"))


def ok(msg):
    print("  [OK]   " + msg)


def bad(msg):
    print("  [FAIL] " + msg)


def info(msg):
    print("  [--]   " + msg)


# --------------------------------------------------------------------------
# check
# --------------------------------------------------------------------------

def cmd_check(args):
    root = resolve_root(args.path)
    print("WorkAgent check —— " + str(root))
    failures = 0

    print("\n[1/4] 必需文件")
    for rel in REQUIRED_FILES:
        if (root / rel).exists():
            ok(rel)
        else:
            bad(rel + " 缺失")
            failures += 1

    print("\n[2/4] JSON 可解析性")
    for rel in REQUIRED_FILES:
        if not rel.endswith(".json"):
            continue
        try:
            read_json(root / rel)
            ok(rel)
        except Exception as exc:
            bad(rel + " -> " + str(exc))
            failures += 1

    print("\n[3/4] 兼容声明")
    try:
        compat = read_json(root / "compat.json")
        matrix = compat.get("capability_matrix", [])
        lost = [c["capability"] for c in matrix if c.get("on_legacy") == "lost"]
        ok("compat.json 已声明 " + str(len(matrix)) + " 项能力")
        info("旧平台将丢失: " + (", ".join(lost) if lost else "无"))
        if not compat.get("degradation"):
            bad("缺少 degradation 段：无法说明降级行为")
            failures += 1
        else:
            ok("已显式声明降级行为")
    except Exception as exc:
        bad("compat.json -> " + str(exc))
        failures += 1

    print("\n[4/4] 边界与测试套件")
    try:
        boundary = read_json(root / "x-workagent/boundary.json")
        if boundary.get("stop_condition"):
            ok("boundary.json 含 stop_condition")
        else:
            bad("boundary.json 缺 stop_condition")
            failures += 1
        import glob as _glob
        suites = sorted(_glob.glob(str(root / "x-workagent/test_suite/*.json")))
        ok("测试套件 " + str(len(suites)) + " 个: " + ", ".join(Path(s).stem for s in suites))
    except Exception as exc:
        bad("boundary/test_suite -> " + str(exc))
        failures += 1

    print("")
    if failures:
        print("结果: 不通过，共 " + str(failures) + " 项问题")
        return 1
    print("结果: 通过。本包结构完整、兼容声明齐备。")
    return 0


# --------------------------------------------------------------------------
# adapt
# --------------------------------------------------------------------------

def cmd_adapt(args):
    root = resolve_root(args.path)
    m = load_manifest(root)
    out = root / "adapters"
    out.mkdir(exist_ok=True)

    plugin = {
        "name": m.get("name") or m.get("id", "workagent").split("/")[-1],
        "version": str(m.get("version", SPEC_VERSION)),
        "description": m.get("summary_en") or m.get("summary", ""),
        "author": (m.get("author") or {}).get("name", ""),
        "license": m.get("license", ""),
        "homepage": "",
        "repository": "",
        "components": {
            "skills": [],
            "mcpServers": {}
        },
        "x-workagent": {
            "manifest": "agent.yaml",
            "compat": "compat.json",
            "boundary": "x-workagent/boundary.json",
            "note": "生成物。真源为根目录 agent.yaml。"
        }
    }
    write_json(out / "plugin.json", plugin)

    card = {
        "name": m.get("display_name_en") or m.get("name", "workagent"),
        "description": m.get("summary_en") or m.get("summary", ""),
        "url": "",
        "version": str(m.get("version", SPEC_VERSION)),
        "protocolVersion": "1.0",
        "preferredTransport": "JSONRPC",
        "defaultInputModes": ["application/json"],
        "defaultOutputModes": ["application/json", "text/markdown"],
        "capabilities": {
            "streaming": False,
            "pushNotifications": False,
            "stateTransitionHistory": True
        },
        "skills": [],
        "x-workagent": {
            "manifest": "agent.yaml",
            "evidence": EVIDENCE_DIR,
            "note": "生成物。真源为根目录 agent.yaml。"
        }
    }
    write_json(out / "agent-card.json", card)

    print("adapt —— 已生成适配层")
    ok("adapters/plugin.json      -> Agent Plugins 1.0")
    ok("adapters/agent-card.json  -> A2A Agent Card")
    info("真源未改动：agent.yaml")
    return 0


# --------------------------------------------------------------------------
# attest
# --------------------------------------------------------------------------

def cmd_attest(args):
    root = resolve_root(args.path)
    m = load_manifest(root)
    executor = ((m.get("execution") or {}).get("executor") or {})
    cmd = executor.get("command") or []

    closure = {
        "schema": "workagent/attest/closure/1",
        "generated_at": now_iso(),
        "principle": "摘要必须覆盖推理函数的完整计算闭包。任何落在被证明集合之外的组件，都是一条伪造通道。",
        "closure": {
            "executor_digest": sha256_file(root / cmd[1]) if len(cmd) > 1 and (root / cmd[1]).exists() else None,
            "inputs_schema_digest": sha256_file(root / "inputs.schema.json"),
            "workflow_digest": sha256_file(root / "execution/workflow.yml"),
            "manifest_digest": sha256_file(root / "agent.yaml"),
            "python_version": platform.python_version(),
            "platform": platform.platform(),
            "numeric_env": {
                "PYTHONHASHSEED": "0",
                "LC_ALL": "C.UTF-8",
                "TZ": "UTC"
            },
            "model_weights_digest": None,
            "tokenizer_digest": None,
            "chat_template_digest": None,
            "engine_build_digest": None,
            "decoding_config": None
        },
        "llm_used": bool(((m.get("execution") or {}).get("llm_steps"))),
        "note": "确定性执行体不涉及模型闭包；一旦启用 LLM 步骤，model/tokenizer/chat_template/engine/decoding 字段必须填写，否则不能声称结论可复现。"
    }
    write_json(root / "attest/closure.json", closure)
    print("attest —— 计算闭包摘要已写入 attest/closure.json")
    c = closure["closure"]
    ok("executor_digest      " + str(c["executor_digest"])[:16])
    ok("inputs_schema_digest " + str(c["inputs_schema_digest"])[:16])
    ok("workflow_digest      " + str(c["workflow_digest"])[:16])
    ok("python_version       " + str(c["python_version"]))
    info("llm_used = " + str(closure["llm_used"]))
    return 0


# --------------------------------------------------------------------------
# run
# --------------------------------------------------------------------------

def run_executor(root, manifest, input_path):
    executor = ((manifest.get("execution") or {}).get("executor") or {})
    cmd = list(executor.get("command") or [])
    if not cmd:
        raise SystemExit("agent.yaml 中未声明 execution.executor.command")
    full = cmd + [str(input_path)]
    proc = subprocess.run(full, cwd=str(root), capture_output=True, text=True,
                          encoding="utf-8", errors="replace")
    if proc.returncode != 0:
        raise SystemExit("执行体失败 (exit " + str(proc.returncode) + "):\n" + proc.stderr)
    try:
        payload = json.loads(proc.stdout)
    except json.JSONDecodeError as exc:
        raise SystemExit("执行体输出不是合法 JSON: " + str(exc) + "\n原始输出:\n" + proc.stdout[:2000])
    return payload.get("output", payload), cmd


def cmd_run(args):
    root = resolve_root(args.path)
    m = load_manifest(root)
    input_path = Path(args.input)
    if not input_path.is_absolute():
        input_path = (root / input_path).resolve()
    if not input_path.exists():
        raise SystemExit("找不到输入文件: " + str(input_path))

    input_obj = read_json(input_path)
    input_digest = sha256_bytes(canonical_json(input_obj).encode("utf-8"))
    output_obj, executor_cmd = run_executor(root, m, input_path)
    output_digest = sha256_bytes(canonical_json(output_obj).encode("utf-8"))

    evidence_id = sha256_bytes((input_digest + output_digest + str(m.get("version"))).encode("utf-8"))[:16]
    ev_dir = root / EVIDENCE_DIR / evidence_id
    ev_dir.mkdir(parents=True, exist_ok=True)

    write_json(ev_dir / "input.json", input_obj)
    write_json(ev_dir / "output.json", output_obj)

    summary_tpl = root / "artifacts/summary.template.md"
    if summary_tpl.exists():
        text = summary_tpl.read_text(encoding="utf-8")
    else:
        text = "# 结论摘要\n\n<conclusion>\n"
    conf = output_obj.get("confidence") or {}
    risks = output_obj.get("risks") or []
    replacements = {
        "<project>": str(output_obj.get("project", "")),
        "<conclusion>": str(output_obj.get("conclusion", "")),
        "<currency>": str(output_obj.get("currency", "")),
        "<total_price>": str(output_obj.get("total_price", "")),
        "<confidence.level>": str(conf.get("level", "unknown")),
        "<confidence.basis>": str(conf.get("basis", "")),
        "<risks>": "\n".join("- " + str(r) for r in risks) if risks else "- （无）",
        "<generated_at>": now_iso(),
        "<executor_digest>": sha256_file(root / executor_cmd[1])[:16] if len(executor_cmd) > 1 else "n/a",
        "<output_digest>": output_digest[:16],
        "<evidence_id>": evidence_id,
    }
    for k, v in replacements.items():
        text = text.replace(k, v)
    (ev_dir / "summary.md").write_text(text, encoding="utf-8")
    (root / "artifacts/summary.md").write_text(text, encoding="utf-8")

    trace = read_json(root / "provenance/trace.json")
    trace["evidence_id"] = evidence_id
    trace.setdefault("steps", [])
    trace["steps"].append({
        "seq": len(trace["steps"]) + 1,
        "at": now_iso(),
        "step": "run",
        "executor": " ".join(executor_cmd),
        "input_file": str(input_path),
        "input_digest": input_digest,
        "output_digest": output_digest,
        "evidence_id": evidence_id
    })
    write_json(root / "provenance/trace.json", trace)

    write_json(ev_dir / "evidence.json", {
        "schema": "workagent/evidence/1",
        "evidence_id": evidence_id,
        "generated_at": now_iso(),
        "spec_version": SPEC_VERSION,
        "tool_version": TOOL_VERSION,
        "executor": " ".join(executor_cmd),
        "input_digest": input_digest,
        "output_digest": output_digest,
        "output": output_obj
    })

    print("run —— 执行完成")
    ok("证据包编号 " + evidence_id)
    ok("结论: " + str(output_obj.get("conclusion", "(无)")))
    ok("输出摘要 " + output_digest[:16])
    info("证据目录 " + str(ev_dir))
    info("摘要文件 " + str(root / "artifacts/summary.md"))
    return 0


# --------------------------------------------------------------------------
# verify
# --------------------------------------------------------------------------

def cmd_verify(args):
    root = resolve_root(args.path)
    m = load_manifest(root)
    trace = read_json(root / "provenance/trace.json")
    runs = [s for s in trace.get("steps", []) if s.get("step") == "run"]
    if not runs:
        print("verify —— 没有可复现的记录，请先运行 workagent run")
        return 1
    last = runs[-1]
    print("verify —— 复现校验，证据包 " + str(last.get("evidence_id")))

    input_file = Path(last["input_file"])
    if not input_file.exists():
        input_file = root / EVIDENCE_DIR / str(last["evidence_id"]) / "input.json"
    if not input_file.exists():
        print("  [FAIL] 找不到原始输入，无法复现")
        return 1

    output_obj, _ = run_executor(root, m, input_file)
    replay_digest = sha256_bytes(canonical_json(output_obj).encode("utf-8"))
    expected = last.get("output_digest")

    print("  期望摘要 " + str(expected)[:32])
    print("  重放摘要 " + str(replay_digest)[:32])
    if replay_digest == expected:
        print("结果: PASS —— 同输入产出同结论，可复现。")
        return 0
    print("结果: FAIL —— 结论不可复现，禁止对外交付该结论。")
    return 1


# --------------------------------------------------------------------------
# seal
# --------------------------------------------------------------------------

def cmd_seal(args):
    root = resolve_root(args.path)
    skip_dirs = {".git", "__pycache__", "out", "adapters", ".venv", "dist", "build"}
    entries = []
    for p in sorted(root.rglob("*")):
        if not p.is_file():
            continue
        rel = p.relative_to(root)
        if any(part in skip_dirs for part in rel.parts):
            continue
        if rel.as_posix() == "signature/agent.sig":
            continue
        entries.append({"path": rel.as_posix(), "sha256": sha256_file(p)})

    package_digest = sha256_bytes("\n".join(e["path"] + ":" + e["sha256"] for e in entries).encode("utf-8"))
    sig = {
        "schema": "workagent/signature/1",
        "generated_at": now_iso(),
        "spec_version": SPEC_VERSION,
        "scheme": "digest-only",
        "signer": None,
        "public_key": None,
        "package_digest": package_digest,
        "file_count": len(entries),
        "files": entries,
        "honesty_note": "digest-only 只证明『内容自生成后未变』，不证明『谁签的』。如需不可否认性，请用 GPG/Ed25519 对 package_digest 另行签名，并把公钥放入 signature/。"
    }
    write_json(root / "signature/agent.sig", sig)
    print("seal —— 全包摘要已生成")
    ok("package_digest " + package_digest)
    ok("文件数 " + str(len(entries)))
    info("scheme = digest-only（未冒充密码学签名）")
    return 0


# --------------------------------------------------------------------------

def main(argv=None):
    parser = argparse.ArgumentParser(prog="workagent", description="WorkAgent 包工具链 v" + TOOL_VERSION)
    parser.add_argument("--version", action="version", version="workagent " + TOOL_VERSION)
    sub = parser.add_subparsers(dest="command")

    p = sub.add_parser("check", help="校验包结构与兼容声明")
    p.add_argument("path", nargs="?", default=".")
    p.set_defaults(func=cmd_check)

    p = sub.add_parser("adapt", help="从 agent.yaml 生成适配层")
    p.add_argument("path", nargs="?", default=".")
    p.set_defaults(func=cmd_adapt)

    p = sub.add_parser("attest", help="计算 Attestation Closure 摘要")
    p.add_argument("path", nargs="?", default=".")
    p.set_defaults(func=cmd_attest)

    p = sub.add_parser("run", help="执行并产出证据包")
    p.add_argument("--input", required=True, help="输入 JSON 路径")
    p.add_argument("path", nargs="?", default=".")
    p.set_defaults(func=cmd_run)

    p = sub.add_parser("verify", help="复现校验")
    p.add_argument("path", nargs="?", default=".")
    p.set_defaults(func=cmd_verify)

    p = sub.add_parser("seal", help="生成全包摘要")
    p.add_argument("path", nargs="?", default=".")
    p.set_defaults(func=cmd_seal)

    args = parser.parse_args(argv)
    if not getattr(args, "func", None):
        parser.print_help()
        return 0
    return args.func(args)


if __name__ == "__main__":
    sys.exit(main())
