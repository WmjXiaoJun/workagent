// Package render 把执行体输出的**结构化事实**渲染成人话。
//
// 分层原则：
//
//	执行体（Python）  →  只输出 code + args，不含任何自然语言文案
//	包内 messages.json →  承载文案，zh/en 并排，缺哪条一眼可见
//	本包（Go）        →  按 --lang 渲染
//
// 为什么文案不放执行体：执行体是独立进程，拿不到 CLI 的语言设置；
// 而且文案属于**内容**，它的真源应该和 agent.yaml 一样留在交付包里。
//
// 关键约束：**渲染绝不参与哈希**。输出摘要取自执行体的原始结构化输出，
// 否则中文跑一次、英文跑一次会得到不同摘要，verify 直接失效。
package render

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// asString 只认字符串，其余一律空串（渲染路径不该被类型问题绊倒）。
func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// deepCopy 深拷贝 JSON 值，保证渲染副本与原始输出完全隔离。
func deepCopy(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[k] = deepCopy(val)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = deepCopy(val)
		}
		return out
	}
	return v
}

func asMapDeepCopy(v any) map[string]any {
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	if cp, ok := deepCopy(m).(map[string]any); ok {
		return cp
	}
	return nil
}

var placeholder = regexp.MustCompile(`\{([a-zA-Z_][a-zA-Z0-9_]*)\}`)

// Catalog 是包级文案表。
type Catalog struct {
	// Messages 的键是 "<caseID>.<CODE>"，值是 {"zh": "...", "en": "..."}。
	Messages map[string]map[string]string `json:"messages"`
	// Verdicts 的键是 caseID → 判定词原文 → {"zh","en"}。
	Verdicts map[string]map[string]map[string]string `json:"verdicts"`
}

// Load 读取包根目录下的 messages.json。
// 文件不存在不是错误：没有文案表时一切照旧，沿用执行体自带的文本。
func Load(root string) *Catalog {
	c := &Catalog{Messages: map[string]map[string]string{}, Verdicts: map[string]map[string]map[string]string{}}
	b, err := os.ReadFile(filepath.Join(root, "messages.json"))
	if err != nil {
		return c
	}
	if err := json.Unmarshal(b, c); err != nil {
		return c
	}
	if c.Messages == nil {
		c.Messages = map[string]map[string]string{}
	}
	if c.Verdicts == nil {
		c.Verdicts = map[string]map[string]map[string]string{}
	}
	return c
}

// Format 用 args 替换模板里的 {name}。缺失的键原样保留，便于发现漏填。
func Format(tpl string, args map[string]any) string {
	return placeholder.ReplaceAllStringFunc(tpl, func(m string) string {
		name := m[1 : len(m)-1]
		v, ok := args[name]
		if !ok {
			return m
		}
		switch n := v.(type) {
		case float64:
			if n == float64(int64(n)) {
				return fmt.Sprintf("%d", int64(n))
			}
			return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.4f", n), "0"), ".")
		case nil:
			return ""
		default:
			return fmt.Sprint(v)
		}
	})
}

func pick(lang string, e map[string]string) (string, bool) {
	if e == nil {
		return "", false
	}
	if lang == "en" {
		if v, ok := e["en"]; ok && v != "" {
			return v, true
		}
		return "", false // 英文缺失就不渲染，回落到执行体自带文本 —— 不冒充已翻译
	}
	if v, ok := e["zh"]; ok && v != "" {
		return v, true
	}
	return "", false
}

// Conclusion 渲染结论句。args 为 nil 或没有对应文案时返回 ok=false。
// Conclusion 渲染结论句。code 为空时用保留码 CONCLUSION。
//
// 执行体可以按**事实**选不同的文案码（例如"触发判异"与"过程受控"），
// 但文案本身仍由本表提供 —— 执行体不写人话这条不变。
func (c *Catalog) Conclusion(lang, caseID, code string, args map[string]any) (string, bool) {
	if c == nil || args == nil {
		return "", false
	}
	if code == "" {
		code = "CONCLUSION"
	}
	tpl, ok := pick(lang, c.Messages[caseID+"."+code])
	if !ok {
		return "", false
	}
	return Format(tpl, args), true
}

// Finding 渲染一条发现。
func (c *Catalog) Finding(lang, caseID, code string, args map[string]any) (string, bool) {
	if c == nil || args == nil || code == "" {
		return "", false
	}
	tpl, ok := pick(lang, c.Messages[caseID+"."+code])
	if !ok {
		return "", false
	}
	return Format(tpl, args), true
}

// Verdict 翻译判定词；没有对应条目时原样返回。
func (c *Catalog) Verdict(lang, caseID, verdict string) string {
	if c == nil {
		return verdict
	}
	if v, ok := pick(lang, c.Verdicts[caseID][verdict]); ok {
		return v
	}
	return verdict
}

// Apply 生成**用于显示**的副本：结论、判定、发现文案按 lang 渲染。
//
// 返回的是深拷贝，调用方务必只在显示/存档路径上用它；
// 证据包里的原始输出保持不动，摘要才对得上。
func (c *Catalog) Apply(lang, caseID string, output any) any {
	m := asMapDeepCopy(output)
	if m == nil {
		return output
	}
	if v := asString(m["verdict"]); v != "" {
		m["verdict"] = c.Verdict(lang, caseID, v)
	}
	if args, ok := m["conclusion_args"].(map[string]any); ok {
		// 结论句里通常嵌着判定词，它也得跟着翻 —— 否则英文句子里夹一个中文词。
		use := args
		if v, isStr := args["verdict"].(string); isStr {
			cp, _ := deepCopy(args).(map[string]any)
			if cp != nil {
				cp["verdict"] = c.Verdict(lang, caseID, v)
				use = cp
			}
		}
		if s, done := c.Conclusion(lang, caseID, asString(m["conclusion_code"]), use); done {
			m["conclusion"] = s
		}
	}
	if fs, ok := m["findings"].([]any); ok {
		rendered := make([]any, len(fs))
		for i, f := range fs {
			fm := asMapDeepCopy(f)
			if fm == nil {
				rendered[i] = f
				continue
			}
			// 没有 args 就没有可渲染的事实 —— 保留原文，不猜。
			if args, hasArgs := fm["args"].(map[string]any); hasArgs {
				if s, done := c.Finding(lang, caseID, asString(fm["code"]), args); done {
					fm["message"] = s
				}
			}
			rendered[i] = fm
		}
		m["findings"] = rendered
	}
	return m
}

// Coverage 报告文案表的覆盖情况，供 check 使用：
// 返回缺失英文的键，以及被引用的执行体场景数。
func (c *Catalog) Coverage() (missingEn []string, total int) {
	for k, e := range c.Messages {
		total++
		if e["en"] == "" {
			missingEn = append(missingEn, k)
		}
		if e["zh"] == "" {
			missingEn = append(missingEn, k)
		}
	}
	sort.Strings(missingEn)
	return missingEn, total
}
