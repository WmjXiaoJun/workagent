# 知识库摄入审计核验

**形态**：核验型 ｜ **复现等级**：L3（纯确定性，无 LLM）

## 规则从哪来

字段与事件类型**取自一份真实运行的审计日志**（22,253 条事件，
带 `old_sha256` / `new_sha256` / `line_count` / `semantic_stale` / `source_changed`）。
示例数据是**合成的**——真实日志含你的文档路径与内容哈希，不入公开仓库。

## 六项核验

| 代号 | 校验 | 级别 |
|---|---|---|
| AC-01 | 同一 `doc_id` 的哈希链：`old_sha256` 必须等于该文档上一次的 `new_sha256` | error |
| AC-02 | `source_changed=true` 时哈希必须真的变了 | error |
| AC-03 | `semantic_stale=true` 时 `semantic_status` 必须标记为需更新 | warn |
| AC-04 | `line_count` 相对上次下降超过阈值（默认 50%）——疑似截断 | error |
| AC-05 | 同一文档的事件时间戳不得倒退 | warn |
| AC-06 | 非摄入事件引用了从未被建立过的 `doc_id` | warn |

### 一个实现上的关键点

**不能按 ts 全局排序后再校验。** 审计日志是追加写的有序流，
按时间戳排序会把"时间戳倒退"这个要检测的现象**直接排掉**——bug 消失得无影无踪。

正确做法：**按文档分组，组内保留日志原始顺序**。这个坑是写第一版时踩到的。

## 实测输出

```text
核验判定「不一致」：事件 9 条 / 文档 4 个，错误 3 项、告警 3 项

[error] AC-01 哈希链断裂：doc=doc_beta id=a04 old=bbbb9999… 应为 bbbb1111…
[error] AC-02 标称源已变更但哈希未变：doc=doc_delta id=a08 sha=dddd1111…
[error] AC-04 行数异常下降：doc=doc_gamma id=a06 上次 300 行 → 本次 90 行（降幅 70%）
[warn]  AC-05 时间戳倒退：doc=doc_beta id=a04 上一条 ts=...300 本条 ts=...250
[warn]  AC-03 标记语义过期但状态未更新：doc=doc_gamma id=a05 semantic_status='fresh'
[warn]  AC-06 引用了未经摄入建立的文档：doc=doc_missing 事件=section.update id=a09
```

六条规则**各命中一次**，没有假阳性。

## 运行

```bash
workagent run --case audit_chain --input examples/audit_chain/input.json .
workagent verify .
```
