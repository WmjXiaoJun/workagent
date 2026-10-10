# 金融 · 项目贷款偿债能力测算

**形态**：测算型 ｜ **复现等级**：L3 ｜ **行业**：金融 / 信贷风控

## 规则

```text
DSCR = (净利润 + 折旧 + 摊销 + 利息支出) / (本金摊还 + 利息支出)
ICR  = EBIT / 利息支出
```

| DSCR | 判定 |
|---|---|
| ≥ 1.25 | 通过 |
| 1.10 ~ 1.25 | 需复核 |
| < 1.10 | 不予通过 |

阈值可在输入中覆盖（`dscr_min` / `dscr_review`），**不写死在代码里**——不同机构的政策不同。

## 运行

```bash
workagent run --case loan_dscr --input examples/loan_dscr/input.json .
workagent verify .
```
