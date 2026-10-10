# 兼容指南

[English](COMPAT.md) · **中文**

> 核心约束：**新能力要上，老平台不能坏。**

## 1. 策略：新增只加目录，不改字段

```text
agent.yaml（真源，字段结构不变）
   │
   ├── APS 基础字段 ────────▶ 任何老平台都能读
   └── x-workagent 扩展区 ──▶ 老平台直接忽略，新平台才解析
```

老平台的行为是**忽略未知顶层键**，因此扩展区不会导致解析失败。这是本方案能兼容的前提。

> ⚠️ 如果你的老平台遇未知字段会**报错而非忽略**，那它不满足 `min_platform` 要求，
> 应在 `compat.json` 中如实声明，或把扩展区拆到单独文件再引用。

## 2. 老平台上会丢什么

以本仓库为例（`compat.json` 的 `capability_matrix`）：

| 能力 | 依赖 | 老平台 |
|---|---|---|
| 读取清单、输入输出校验、工作流执行、`summary.md` | APS | 保留 |
| 失效边界校验 | `x-workagent` | **丢失** |
| 回归基准比对 | `x-workagent` | **丢失** |
| 结论可复现校验 | `x-workagent` + `attest` | **丢失** |
| 包完整性验签 | `x-workagent` + `signature` | **丢失** |

## 3. 怎么声明降级

`compat.json` 的 `degradation.legacy_platform` 必须写清三件事：

1. `preserved` —— 还剩下什么；
2. `lost` —— 丢了什么；
3. `statement` —— **一句给最终用户看的话**，说明这意味什么。

参考写法：

> 在仅支持 APS 基础字段的平台上，本包可以运行并产出结论；但该结论不可复现校验、不可验签、不受失效边界保护。请勿据此结论做高风险决策。

**这句话是本项目最被低估的部分。** 大多数项目让用户自己猜，我们把它写进机器可读的文件里。

## 4. 从 v0.2 迁移到 v0.3

| v0.2 | v0.3 | 动作 |
|---|---|---|
| `agent.yaml` | `agent.yaml` | 保留，**不改字段结构** |
| —— | `compat.json` | 新增 |
| —— | `attest/closure.json` | 新增 |
| `artifacts/summary.md` | `summary.md` + `summary.template.md` | 拆分模板与产物 |
| `provenance/trace.json` | 同 | 补 `input_digest` / `output_digest` / `evidence_id` |
| `signature/agent.sig` | 同 | 明确 `scheme` 字段，禁止含糊表述 |
| —— | `adapters/` | 新增（生成物） |

**迁移不破坏老平台**：全部是新增文件，既有字段语义未变。

## 5. 与其它生态的适配

`workagent adapt` 生成：

| 生成物 | 目标 | 说明 |
|---|---|---|
| `adapters/plugin.json` | Agent Plugins 1.0 | 可移植插件清单 |
| `adapters/agent-card.json` | A2A | Agent 能力自描述，供发现 |

**注意**：这些是**适配**，不是**遵循**。本项目不声称符合上述规范的**全部**要求（例如 Agent Plugins 规定 manifest 为 `plugin.json`，而我们的真源是 `agent.yaml`，适配层只做映射）。对外表述时请说"**可导出为……格式**"，不要写"**符合……标准**"。

## 6. 反模式

| 反模式 | 为什么错 | 正确做法 |
|---|---|---|
| 直接改 `adapters/` | 下次生成被覆盖，且真源与适配层不一致 | 改 `agent.yaml`，重新 `adapt` |
| 为了对齐新标准改 `agent.yaml` 字段名 | 破坏老平台兼容 | 用适配层做映射 |
| `compat.json` 只写"完全兼容" | 用户不知道会丢什么 | 显式列出 `lost` |
| 没有私钥却写"已签名" | 虚假声明，比不签名更糟 | `scheme: digest-only` |
| 用了 LLM 却不登记模型摘要 | 结论不可复现却声称可复现 | 填满 closure，或诚实标注不可复现 |
