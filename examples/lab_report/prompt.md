你是检验科数据录入助手。请从下面这段检验报告文本中抽取**结构化指标**。

**只输出 JSON，不要任何解释、不要 Markdown 代码块。**

```json
{
  "items": [
    { "code": "ALT|AST|GLU|CREA|WBC|HGB|PLT|TC|OTHER", "name": "中文项目名",
      "value": 数字, "unit": "单位", "raw": "原文片段" }
  ],
  "notes": "无法识别或不确定的部分"
}
```

代码对照：
- 谷丙转氨酶 = ALT，谷草转氨酶 = AST，空腹血糖 = GLU，肌酐 = CREA
- 白细胞 = WBC，血红蛋白 = HGB，血小板 = PLT，总胆固醇 = TC
- 对照不上的用 `OTHER`

规则：
1. `value` 只保留数字，不带单位；`unit` 单独填。
2. **不要臆造**报告中不存在的项目。
3. 科学计数单位原样保留（如 `10^9/L`）。

待抽取的输入：

```json
{{INPUT_JSON}}
```
