# 固件资源预算测算（Flash / RAM / 任务栈）

**形态**：测算型 ｜ **复现等级**：L3（纯确定性，无 LLM）

## 检查项

| 代号 | 检查 | 级别 |
|---|---|---|
| FR-01 | Flash 占用超过容量 | error |
| FR-02 | Flash 占用率超过告警阈值（默认 85%） | warn |
| FR-03 | RAM 占用超过容量 | error |
| FR-04 | RAM 占用率超过告警阈值（默认 80%） | warn |
| FR-05 | 任务栈余量低于安全下限（默认 20%） | error |
| FR-06 | 栈峰值超过分配量（已经溢出） | error |

**FR-05 是最有价值的一条**：栈溢出是现场最难查的故障，症状千奇百怪。
把 RTOS 的栈高水位实测值喂进来，余量不足直接报错，不用等到现场炸。

## 输出

- 总占用与余量（Flash / RAM 分别）
- 占用最大的前 5 个模块（便于定位优化对象）
- 每个任务的栈余量，按**余量从小到大**排序

## 运行

```bash
workagent run --case firmware_budget --input examples/firmware_budget/input.json .
workagent verify .
```

示例数据里 Flash 占用 910/1024 KB（约 88.9%，触发 FR-02 告警）、
通信任务栈余量约 16%（低于 20%，触发 FR-05 错误）—— 两个都是真实项目里常见的状态。
