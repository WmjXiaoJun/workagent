// Package i18n 提供 CLI 输出的中英双语支持。
//
// 设计选择：**不做 key 目录，用内联中英对** —— i18n.T("中文", "English")。
// 理由：
//   - 没有 key 就没有「key 拼错」「忘了加翻译」这类错误；
//   - 中文留在调用点，读代码的人一眼看到实际会输出什么；
//   - 改动是机械的，不需要先抽一遍 key 再回填。
//
// 代价：同一条文案多处出现时要改多处。对本项目这点重复是可接受的。
package i18n

import (
	"fmt"
	"os"
	"strings"
)

const (
	// LangZh 中文（默认）—— 这是使用者日常看输出用的语言。
	LangZh = "zh"
	// LangEn 英文 —— 公开演示、给非中文读者。
	LangEn = "en"
)

var lang = LangZh

// SetLang 设置当前语言。只有 "en" 前缀会被识别为英文，其余一律中文。
func SetLang(l string) {
	l = strings.ToLower(strings.TrimSpace(l))
	if strings.HasPrefix(l, "en") {
		lang = LangEn
		return
	}
	lang = LangZh
}

// Lang 返回当前语言。
func Lang() string { return lang }

// IsEn 当前是否为英文。
func IsEn() bool { return lang == LangEn }

// T 在中英两条文案里按当前语言选一条。
func T(zh, en string) string {
	if lang == LangEn {
		return en
	}
	return zh
}

// Tf 带格式化的 T。注意中英两条的占位符个数与类型必须一致。
func Tf(zh, en string, args ...any) string {
	if lang == LangEn {
		return fmt.Sprintf(en, args...)
	}
	return fmt.Sprintf(zh, args...)
}

// Resolve 解析语言设置：--lang <v> / --lang=<v> 优先，其次 WORKAGENT_LANG，默认中文。
func Resolve(env string, args []string) string {
	l := env
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--lang" || a == "-lang":
			if i+1 < len(args) {
				l = args[i+1]
			}
		case strings.HasPrefix(a, "--lang="):
			l = strings.TrimPrefix(a, "--lang=")
		}
	}
	return l
}

// StripLang 从参数里移除 --lang 及其取值，避免它被当成位置参数。
// （不这么做的话 `run --case X --lang en .` 里的 en 会变成位置参数。）
func StripLang(args []string) []string {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--lang" || a == "-lang":
			i++ // 跳过取值
		case strings.HasPrefix(a, "--lang="):
			// 丢弃
		default:
			out = append(out, a)
		}
	}
	return out
}

// Shape 把交付形态词表翻成英文。
//
// 形态是**规范级词汇**（固定五种，见 docs/VERTICALS.md），所以映射放在这里；
// 而场景标题是内容，它的真源是 agent.yaml —— 那边用 title_en 字段承载。
func Shape(s string) string {
	if !IsEn() {
		return s
	}
	switch s {
	case "测算型":
		return "Estimation"
	case "核验型":
		return "Verification"
	case "抽取型":
		return "Extraction"
	case "排程型":
		return "Scheduling"
	case "推演型":
		return "Simulation"
	case "未分类":
		return "Unclassified"
	}
	return s
}

// Shapes 返回当前语言下的形态顺序（固定：测算 → 核验 → 抽取 → 排程 → 推演）。
func Shapes() []string {
	if IsEn() {
		return []string{"Estimation", "Verification", "Extraction", "Scheduling", "Simulation"}
	}
	return []string{"测算型", "核验型", "抽取型", "排程型", "推演型"}
}

// EnvLang 读取环境变量（单独抽出来便于测试）。
func EnvLang() string { return os.Getenv("WORKAGENT_LANG") }
