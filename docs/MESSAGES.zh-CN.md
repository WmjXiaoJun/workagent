# 文案表（messages.json）

**执行体不写人话，人话在这里。**

## 为什么要这么分

执行体是**独立进程**，拿不到 CLI 的语言设置。更要紧的是：文案属于**内容**，
它的真源应该和 `agent.yaml` 一样留在交付包里，而不是散在 12 个 Python 文件里。

```text
执行体（Python）    →  只输出 code + args，不含任何自然语言
messages.json       →  承载文案，zh / en 并排，缺哪条一眼可见
workagent（Go）     →  按 --lang 渲染
```

## 一条铁律：渲染绝不参与哈希

`output_digest` 取自**执行体的原始结构化输出**，渲染发生在取摘要**之后**。

如果渲染结果进了哈希，中文跑一次、英文跑一次就会得到两个不同的摘要，`verify` 立刻失效。
实测证据：

```text
$ workagent run --case audit_chain .
  证据包编号 ed6532b74aec41d1
$ workagent run --case audit_chain . --lang en
  Evidence bundle id ed6532b74aec41d1      ← 同一个
```

证据包里的 `output.json` 保持原始输出；给人看的是 `summary.md` 和 `records/`。

## 格式

```json
{
  "messages": {
    "audit_chain.CONCLUSION": {
      "zh": "核验判定「{verdict}」：事件 {events} 条 / 文档 {docs} 个",
      "en": "Audit verdict \"{verdict}\": {events} events / {docs} documents"
    },
    "audit_chain.AC-01": { "zh": "...", "en": "..." }
  },
  "verdicts": {
    "audit_chain": { "不一致": { "zh": "不一致", "en": "inconsistent" } }
  }
}
```

- 键 `<caseID>.<CODE>`；`CONCLUSION` 是保留码，用于结论句。
- 占位符 `{name}` 由 `args` 填充。**缺失的键原样保留**，便于发现漏填。
- 结论句里的 `{verdict}` 会先经 `verdicts` 翻译，避免英文句子里夹一个中文词。
- **英文缺失就不渲染**，回落到执行体自带原文 —— 不冒充已翻译。`check` 会因此报错。

## 执行体要做什么

发现里加 `args`，输出里加 `conclusion_args`：

```python
findings.append({
    "level": "error", "code": "AC-01",
    "args": {"doc": did, "id": eid, "got": old[:12], "want": last_hash[did][:12]},
    "message": "哈希链断裂：" + did,        # 兼容回退：老消费者只读 message
})

out["conclusion_args"] = {"verdict": verdict, "events": n, "docs": m, ...}
```

`message` **保留**：规范里已有该字段，删掉是破坏性变更。带 `args` 时渲染层优先用
`args` + 文案表；没有 `args` 时原样显示 `message`。

## 一个 code 只能有一种文案

文案表是 `code → 模板` 的映射，所以**同一个 code 不能承载两种不同的话**。

`audit_chain` 最初把「哈希链断裂」和「首次摄入却带 old_sha256」都记成 `AC-01` ——
后者被渲染成前者的文案，**错得毫无征兆**。已拆成 `AC-01` 与 `AC-01F`。
