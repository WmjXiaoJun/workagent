# 示例：投标报价测算

本示例用于验证 WorkAgent 的最小闭环：**输入 → 结论 → 证据包**。

## 运行

```bash
python tools/workagent.py run --input examples/bid_quote/input.json .
python tools/workagent.py verify .
```

## 为什么这个例子是"可验证"的

| 条件 | 本示例 |
|---|---|
| 执行体确定性（无随机、无网络、无时间依赖） | 是 |
| 输入输出有 JSON Schema | 是 |
| 结论可通过重放比对摘要验证 | 是（`verify`） |
| 假设与边界显式声明 | 是（`provenance/assumptions.json`、`x-workagent/boundary.json`） |
| 越界输入会拒绝而非硬算 | 是（`stop_condition`） |

## 换成你自己的场景

1. 复制本目录为 `examples/your_case/`；
2. 改 `executor.py`（保持契约：读 argv[1]，向 stdout 输出 `{"output": {...}}`）；
3. 改根目录 `inputs.schema.json` 与 `artifacts/output_schema.json`；
4. 改 `agent.yaml` 里的 `execution.executor.command`；
5. 重跑 `check / adapt / attest / run / verify / seal`。
