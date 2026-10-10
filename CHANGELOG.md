# Changelog

本项目遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

## [0.4.0] - 2026-10-10

**工具链从 Python 重写为 Go。**

### Changed
- `tools/workagent.py` → `cmd/workagent/`（`main.go` + `yaml.go`）
- 分发方式：由"需要 Python 运行时"变为**单个静态二进制**，支持 `GOOS`/`GOARCH` 交叉编译
- `attest/closure.json` 的 `python_version` 字段改为 `toolchain`（如实记录构建工具链）+ 新增 `executor_command`
- Dockerfile 改为两阶段构建：Go 静态二进制 + 执行体运行时

### Added
- `Makefile`：`make release` 一条命令产出五平台二进制
- CI 扩展到三操作系统矩阵

### Why Go
本工具是 I/O 密集型（读文件、算 SHA-256、起子进程），Rust 的性能优势用不上，
却要引入 `serde@@ / `sha2@@ 等 crate，违背"零第三方依赖"这条铁律。
Go 的标准库覆盖全部需求，同时解决最大的痛点：**收到包的人不需要装任何运行时就能验证。**

### Compatibility
重写后**摘要算法与 Python 版完全一致** —— 用同一个输入验证，证据包编号与结论摘要逐字节相同
（`6723c555d11c0a3e` / `8e38afbabb710b00...`）。既有证据包无需重建。

> 已删除 `tools/workagent.py`。**刻意不同时保留两个实现**——两份实现一旦行为分叉，
> 对一个"可复现"项目是致命的。历史版本见 commit `0c77582`。

## [0.3.0] - 2026-10-10

个人可验证版首发。

### Added
- `agent.yaml` 单一真源清单（APS 基础字段 + `x-workagent` 扩展区）
- `compat.json` **兼容降级声明**：显式说明旧平台会丢失哪些能力（本项目的差异化能力）
- `attest/closure.json` **Attestation Closure**：计算闭包摘要，覆盖执行体 / schema / 工作流 / 运行环境
- 零依赖工具链：`check` / `adapt` / `attest` / `run` / `verify` / `seal`
- `adapters/` 适配层生成：Agent Plugins 1.0 `plugin.json` + A2A `agent-card.json`
- `provenance/` 溯源三件套：`trace.json` / `data_sources.json` / `assumptions.json`
- `x-workagent/` 扩展层：`boundary.json` + 四类测试套件
- 示例场景：投标报价测算（`examples/bid_quote/`）
- `docs/SPEC.md` 规范说明、`docs/COMPAT.md` 兼容指南

### Design notes
- 确定性优先：示例执行体不含 LLM；一旦启用 LLM，必须在 closure 中登记模型摘要
- `seal` 默认 `digest-only`，不冒充密码学签名

## [0.2.0] - 规范草案

- 来自《工作智能体（WorkAgent）定制开发指南（兼容规范 v0.2）》
