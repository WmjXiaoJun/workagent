# 财务 · 报销单合规审核

**形态**：核验型 ｜ **复现等级**：L2 / L1 ｜ **行业**：财务 / 行政

模型只把报销说明读成结构化明细；**合规判定全部由确定性代码完成**。

## 检查项

| 代号 | 检查 | 级别 |
|---|---|---|
| EXP-01 | 申报总额与明细合计不一致 | error |
| EXP-02 | 单笔超过政策上限 | error |
| EXP-03 | 超过类别限额（餐饮 1000 / 办公 500 / 住宿 600） | error |
| EXP-04 | 发生日期晚于提交日期，或超过报销时限 | error |
| EXP-05 | 类别不在白名单 | warn |
| EXP-06 | 缺少发票号 | warn |
| EXP-07 | 模型标注了不确定项 | warn |

## 运行

```bash
# 首次：真实调用模型并录制
workagent run --case expense_audit --input examples/expense_audit/input.natural.json .
# 之后（含 CI）：离线重放，无需 key
workagent run --case expense_audit --input examples/expense_audit/input.natural.json . --replay
workagent verify .
```

示例数据故意包含 3 类问题：申报总额 3120 与明细合计 3260 不符、餐饮 1180 超类别限额、部分项缺发票号。
