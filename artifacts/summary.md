# 结论摘要

> 本文件由 `workagent run` 自动生成，是**人工可读、可签字、可归档**的最终摘要。
> 它不堆砌过程数据，只保留核心结论、发现、风险说明与使用建议。

| 项 | 内容 |
|---|---|
| 场景 | `audit_chain` · 知识库摄入审计核验 |
| 对象 | 示例知识库审计日志（字段取自真实 audit.jsonl，数据为合成） 一致性核验 |
| **结论** | **核验判定「不一致」：事件 9 条 / 文档 4 个，错误 3 项、告警 3 项** |
| 判定 | 不一致 |
| 置信度 | high —— 六项校验全部基于事件字段的确定性语义，不依赖模型；字段定义取自真实审计日志 |

## 发现

- **error** [AC-01] 哈希链断裂：doc=doc_beta id=a04 old_sha256=bbbb9999… 应为上一条 new_sha256=bbbb1111…
- **error** [AC-02] 标称源已变更但哈希未变：doc=doc_delta id=a08 sha=dddd1111…
- **error** [AC-04] 行数异常下降：doc=doc_gamma id=a06 上次 300 行 → 本次 90 行（降幅 70%）
- **warn** [AC-05] 时间戳倒退：doc=doc_beta id=a04 上一条 ts=1788185300.000 本条 ts=1788185250.000
- **warn** [AC-03] 标记语义过期但状态未更新：doc=doc_gamma id=a05 semantic_status='fresh'
- **warn** [AC-06] 引用了未经摄入建立的文档：doc=doc_missing 事件=section.update id=a09

## 风险说明

- 只校验事件序列内部自洽，不校验文档内容是否正确
- 哈希链按 doc_id 分组，若同一文档跨多个来源路径需先统一 doc_id 口径
- line_count 突降也可能是合法的文档精简，需人工确认

## 使用建议

1. 本结论仅在 `cases/audit_chain/boundary.json` 声明的适用范围内有效。
2. 引用本结论时，请同时引用证据包编号 `4c332461d3bde931`。
3. 在仅支持 APS 基础字段的平台上运行时，上述可复现性与边界保证**不成立**，详见 `compat.json`。
4. 判定规则与阈值的来源，见 `cases/audit_chain/README.md`。

---
*复现等级：L3-deterministic ｜ 生成时间：2026-10-10T22:37:23+08:00*
*执行体摘要：2619f047c53eb04b ｜ 结论摘要：effc61da775b4443 ｜ 证据包：4c332461d3bde931*
