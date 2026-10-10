package main

// 极简 YAML 子集解析器。
// 只覆盖本规范用到的语法（块映射、块序列、行内列表/映射、标量、注释），
// 目的是让工具链保持零第三方依赖。

import (
	"strconv"
	"strings"
)

type yamlLine struct {
	indent int
	text   string
}

// stripComment 去掉行尾注释，但不动引号内的 #。
func stripComment(line string) string {
	var b strings.Builder
	var quote byte
	for i := 0; i < len(line); i++ {
		c := line[i]
		if quote != 0 {
			b.WriteByte(c)
			if c == quote {
				quote = 0
			}
			continue
		}
		if c == '"' || c == '\'' {
			quote = c
			b.WriteByte(c)
			continue
		}
		if c == '#' {
			break
		}
		b.WriteByte(c)
	}
	return b.String()
}

// splitTop 按顶层逗号切分，忽略引号与括号内的逗号。
func splitTop(s string) []string {
	var parts []string
	var buf strings.Builder
	depth := 0
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			buf.WriteByte(c)
			if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '"', '\'':
			quote = c
			buf.WriteByte(c)
		case '[', '{':
			depth++
			buf.WriteByte(c)
		case ']', '}':
			depth--
			buf.WriteByte(c)
		case ',':
			if depth == 0 {
				parts = append(parts, buf.String())
				buf.Reset()
			} else {
				buf.WriteByte(c)
			}
		default:
			buf.WriteByte(c)
		}
	}
	if strings.TrimSpace(buf.String()) != "" {
		parts = append(parts, buf.String())
	}
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parseScalar(s string) any {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		first := s[0]
		last := s[len(s)-1]
		if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
			return s[1 : len(s)-1]
		}
	}
	if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
		inner := strings.TrimSpace(s[1 : len(s)-1])
		if inner == "" {
			return []any{}
		}
		list := []any{}
		for _, p := range splitTop(inner) {
			list = append(list, parseScalar(p))
		}
		return list
	}
	if strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}") {
		inner := strings.TrimSpace(s[1 : len(s)-1])
		m := map[string]any{}
		for _, p := range splitTop(inner) {
			k, v, _ := strings.Cut(p, ":")
			m[strings.TrimSpace(k)] = parseScalar(v)
		}
		return m
	}
	switch strings.ToLower(s) {
	case "true":
		return true
	case "false":
		return false
	case "null", "~", "":
		return nil
	}
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	return s
}

// parseYAML 返回 map[string]any / []any / string / bool / int64 / float64 / nil
func parseYAML(src string) any {
	var lines []yamlLine
	for _, raw := range strings.Split(src, "\n") {
		line := strings.TrimRight(stripComment(raw), " \t\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		lines = append(lines, yamlLine{indent, strings.TrimSpace(line)})
	}
	if len(lines) == 0 {
		return nil
	}
	pos := 0
	return parseBlock(lines, &pos, lines[0].indent)
}

func parseBlock(lines []yamlLine, pos *int, indent int) any {
	if *pos >= len(lines) {
		return nil
	}
	if lines[*pos].indent == indent && strings.HasPrefix(lines[*pos].text, "- ") {
		list := []any{}
		for *pos < len(lines) && lines[*pos].indent == indent && strings.HasPrefix(lines[*pos].text, "- ") {
			body := strings.TrimSpace(lines[*pos].text[2:])
			*pos = *pos + 1
			if k, v, hasColon := strings.Cut(body, ":"); hasColon && !strings.HasPrefix(body, "{") {
				node := map[string]any{}
				key := strings.TrimSpace(k)
				if strings.TrimSpace(v) != "" {
					node[key] = parseScalar(v)
				}
				if *pos < len(lines) && lines[*pos].indent > indent {
					child := parseBlock(lines, pos, lines[*pos].indent)
					if _, exists := node[key]; !exists {
						node[key] = child
					} else if cm, isMap := child.(map[string]any); isMap {
						for ck, cv := range cm {
							node[ck] = cv
						}
					}
				}
				list = append(list, node)
			} else {
				list = append(list, parseScalar(body))
			}
		}
		return list
	}
	m := map[string]any{}
	for *pos < len(lines) && lines[*pos].indent == indent && !strings.HasPrefix(lines[*pos].text, "- ") {
		line := lines[*pos].text
		k, v, _ := strings.Cut(line, ":")
		key := strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		*pos = *pos + 1
		if v == "" {
			if *pos < len(lines) && lines[*pos].indent > indent {
				m[key] = parseBlock(lines, pos, lines[*pos].indent)
			} else {
				m[key] = nil
			}
		} else {
			m[key] = parseScalar(v)
		}
	}
	return m
}
