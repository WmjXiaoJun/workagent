你是工程造价助理。请从下面这段自然语言描述中抽取**成本明细**。

**只输出 JSON，不要任何解释、不要 Markdown 代码块。**

输出格式（严格遵守）：

```json
{
  "line_items": [
    { "name": "项目名称", "quantity": 数字, "unit_cost": 数字, "category": "material|labor|logistics|other" }
  ],
  "currency": "CNY",
  "notes": "抽取过程中不确定或未识别的部分"
}
```

规则：
1. `quantity` 是数量，`unit_cost` 是单价。只给了总价时，`quantity` 填 1、`unit_cost` 填总价。
2. 金额单位统一为人民币**元**，不要千分位、不要货币符号。
3. **不要臆造**描述中不存在的项目。
4. 无法确定单价的项目放进 `notes`，不要放进 `line_items`。
5. `category` 只能取下列之一：`material`（材料）、`labor`（人工/加工/安装）、`logistics`（运输）、`other`（其它）。

待抽取的输入：

```json
{{INPUT_JSON}}
```
