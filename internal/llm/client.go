package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/WmjXiaoJun/workagent/internal/i18n"
)

// Message 是一条对话消息。
type Message struct {
	Role    string
	Content string
}

// Request 是一次模型调用请求。
type Request struct {
	Model       string
	System      string
	Messages    []Message
	Temperature float64
	MaxTokens   int
	JSONMode    bool
	Seed        int
}

// Response 是一次模型调用结果。
type Response struct {
	Text      string
	Provider  string
	Model     string
	Usage     map[string]any
	Raw       string
	LatencyMS int64
	Replayed  bool
	At        string
}

// Descriptor 描述本次调用的可复现性参数，用于写入 closure。
type Descriptor struct {
	Provider    string
	Model       string
	Kind        string
	Endpoint    string
	Temperature float64
	MaxTokens   int
	Seed        int
	JSONMode    bool
	Replayed    bool
}

var httpClient = &http.Client{Timeout: 180 * time.Second}

func doJSON(ctx context.Context, method, url string, headers map[string]string, body any) (map[string]any, string, error) {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return nil, "", err
		}
		reader = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	text := strings.TrimSpace(string(raw))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		preview := text
		if len(preview) > 400 {
			preview = preview[:400]
		}
		return nil, text, fmt.Errorf("HTTP %d: %s", resp.StatusCode, preview)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, text, fmt.Errorf(i18n.T("响应不是合法 JSON: %v", "response is not valid JSON: %v"), err)
	}
	return out, text, nil
}

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// dig 按路径取值，支持 map 键与数组下标（如 choices/0/message/content）。
func dig(root any, path ...string) any {
	var cur any = root
	for _, key := range path {
		switch node := cur.(type) {
		case map[string]any:
			cur = node[key]
		case []any:
			idx, err := strconv.Atoi(key)
			if err != nil || idx < 0 || idx >= len(node) {
				return nil
			}
			cur = node[idx]
		default:
			return nil
		}
	}
	return cur
}

// Chat 按 profile 名调用模型。model 为空时使用 profile 默认模型。
func Chat(ctx context.Context, providerName, model string, req Request) (*Response, Descriptor, error) {
	p, ok := Resolve(providerName)
	if !ok {
		return nil, Descriptor{}, fmt.Errorf(i18n.T("未知 provider: %s（用 workagent models 查看全部）", "unknown provider: %s (see all with: workagent models)"), providerName)
	}
	if model == "" {
		model = p.DefaultModel
	}
	if model == "" {
		return nil, Descriptor{}, fmt.Errorf(i18n.T("provider %s 需要显式指定 --model", "provider %s requires an explicit --model"), p.Name)
	}
	desc := Descriptor{
		Provider:    p.Name,
		Model:       model,
		Kind:        p.Kind,
		Endpoint:    p.Endpoint(),
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Seed:        req.Seed,
		JSONMode:    req.JSONMode,
	}
	start := time.Now()
	var resp *Response
	var err error
	switch p.Kind {
	case "anthropic":
		resp, err = callAnthropic(ctx, p, model, req)
	case "gemini":
		resp, err = callGemini(ctx, p, model, req)
	default:
		resp, err = callOpenAICompatible(ctx, p, model, req)
	}
	if err != nil {
		return nil, desc, err
	}
	resp.Provider = p.Name
	resp.Model = model
	resp.LatencyMS = time.Since(start).Milliseconds()
	resp.At = time.Now().Format(time.RFC3339)
	return resp, desc, nil
}

func callOpenAICompatible(ctx context.Context, p Profile, model string, req Request) (*Response, error) {
	msgs := []map[string]any{}
	if req.System != "" {
		msgs = append(msgs, map[string]any{"role": "system", "content": req.System})
	}
	for _, m := range req.Messages {
		msgs = append(msgs, map[string]any{"role": m.Role, "content": m.Content})
	}
	body := map[string]any{
		"model":       model,
		"messages":    msgs,
		"temperature": req.Temperature,
	}
	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}
	if req.Seed != 0 {
		body["seed"] = req.Seed
	}
	if req.JSONMode {
		body["response_format"] = map[string]any{"type": "json_object"}
	}
	headers := map[string]string{}
	if key := p.APIKey(); key != "" {
		headers["Authorization"] = "Bearer " + key
	}
	out, raw, err := doJSON(ctx, http.MethodPost, p.Endpoint()+"/chat/completions", headers, body)
	if err != nil {
		return nil, err
	}
	text := asString(dig(out, "choices", "0", "message", "content"))
	if text == "" {
		// 少数厂商返回数组形式的 content
		if arr, ok := dig(out, "choices", "0", "message", "content").([]any); ok {
			var sb strings.Builder
			for _, part := range arr {
				sb.WriteString(asString(dig(part.(map[string]any), "text")))
			}
			text = sb.String()
		}
	}
	usage, _ := out["usage"].(map[string]any)
	return &Response{Text: strings.TrimSpace(text), Usage: usage, Raw: raw}, nil
}

func callAnthropic(ctx context.Context, p Profile, model string, req Request) (*Response, error) {
	if p.APIKey() == "" {
		return nil, fmt.Errorf(i18n.T("provider %s 需要环境变量 %s", "provider %s requires environment variable %s"), p.Name, p.KeyEnv)
	}
	msgs := []map[string]any{}
	for _, m := range req.Messages {
		msgs = append(msgs, map[string]any{"role": m.Role, "content": m.Content})
	}
	body := map[string]any{
		"model":       model,
		"messages":    msgs,
		"max_tokens":  orInt(req.MaxTokens, 4096),
		"temperature": req.Temperature,
	}
	if req.System != "" {
		body["system"] = req.System
	}
	headers := map[string]string{
		"x-api-key":         p.APIKey(),
		"anthropic-version": "2023-06-01",
	}
	out, raw, err := doJSON(ctx, http.MethodPost, p.Endpoint()+"/messages", headers, body)
	if err != nil {
		return nil, err
	}
	var sb strings.Builder
	if arr, ok := out["content"].([]any); ok {
		for _, part := range arr {
			if mm, ok := part.(map[string]any); ok {
				sb.WriteString(asString(mm["text"]))
			}
		}
	}
	usage, _ := out["usage"].(map[string]any)
	return &Response{Text: strings.TrimSpace(sb.String()), Usage: usage, Raw: raw}, nil
}

func callGemini(ctx context.Context, p Profile, model string, req Request) (*Response, error) {
	if p.APIKey() == "" {
		return nil, fmt.Errorf(i18n.T("provider %s 需要环境变量 %s", "provider %s requires environment variable %s"), p.Name, p.KeyEnv)
	}
	contents := []map[string]any{}
	for _, m := range req.Messages {
		role := "user"
		if m.Role == "assistant" {
			role = "model"
		}
		contents = append(contents, map[string]any{
			"role":  role,
			"parts": []map[string]any{{"text": m.Content}},
		})
	}
	gen := map[string]any{"temperature": req.Temperature}
	if req.MaxTokens > 0 {
		gen["maxOutputTokens"] = req.MaxTokens
	}
	if req.JSONMode {
		gen["responseMimeType"] = "application/json"
	}
	body := map[string]any{"contents": contents, "generationConfig": gen}
	if req.System != "" {
		body["systemInstruction"] = map[string]any{"parts": []map[string]any{{"text": req.System}}}
	}
	url := p.Endpoint() + "/models/" + model + ":generateContent?key=" + p.APIKey()
	out, raw, err := doJSON(ctx, http.MethodPost, url, nil, body)
	if err != nil {
		return nil, err
	}
	var sb strings.Builder
	if arr, ok := dig(out, "candidates", "0", "content", "parts").([]any); ok {
		for _, part := range arr {
			if mm, ok := part.(map[string]any); ok {
				sb.WriteString(asString(mm["text"]))
			}
		}
	}
	usage, _ := out["usageMetadata"].(map[string]any)
	return &Response{Text: strings.TrimSpace(sb.String()), Usage: usage, Raw: raw}, nil
}

func orInt(v, def int) int {
	if v > 0 {
		return v
	}
	return def
}
