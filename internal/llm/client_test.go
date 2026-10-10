package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// 这三组测试覆盖三个协议族（OpenAI 兼容 / Anthropic / Gemini）。
// 用 httptest 造假服务端，因此**不需要任何 API key、不联网**，也能在 CI 里跑。
// 这就是"适配主流模型"的可验证证据：请求怎么发、响应怎么解，都是被断言过的。

func TestOpenAICompatibleChat(t *testing.T) {
	var gotAuth, gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, "{\"choices\":[{\"message\":{\"role\":\"assistant\",\"content\":\"{\\\"line_items\\\":[]}\"}}],\"usage\":{\"prompt_tokens\":11,\"completion_tokens\":7}}")
	}))
	defer srv.Close()

	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("OPENAI_BASE_URL", srv.URL+"/v1")

	resp, desc, err := Chat(context.Background(), "openai", "gpt-4o-mini", Request{
		System:      "你是助手",
		Messages:    []Message{{Role: "user", Content: "抽取"}},
		Temperature: 0,
		JSONMode:    true,
	})
	if err != nil {
		t.Fatalf("Chat 失败: %v", err)
	}
	if desc.Provider != "openai" || desc.Model != "gpt-4o-mini" || desc.Kind != "openai" {
		t.Fatalf("descriptor 不对: %+v", desc)
	}
	if gotAuth != "Bearer test-key" {
		t.Errorf("Authorization 头不对: %q", gotAuth)
	}
	if !strings.HasSuffix(gotPath, "/chat/completions") {
		t.Errorf("路径不对: %q", gotPath)
	}
	if gotBody["model"] != "gpt-4o-mini" {
		t.Errorf("model 字段不对: %v", gotBody["model"])
	}
	if _, ok := gotBody["response_format"]; !ok {
		t.Errorf("JSONMode 打开时应带 response_format")
	}
	msgs, _ := gotBody["messages"].([]any)
	if len(msgs) != 2 {
		t.Fatalf("messages 应有 system + user 两条，实际 %d", len(msgs))
	}
	if resp.Text != "{\"line_items\":[]}" {
		t.Errorf("content 解析错误: %q", resp.Text)
	}
	if resp.Usage["prompt_tokens"] == nil {
		t.Errorf("usage 未解析")
	}
}

func TestAnthropicChat(t *testing.T) {
	var gotKey, gotVersion string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("x-api-key")
		gotVersion = r.Header.Get("anthropic-version")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, "{\"content\":[{\"type\":\"text\",\"text\":\"结论：通过\"}],\"usage\":{\"input_tokens\":9,\"output_tokens\":4}}")
	}))
	defer srv.Close()

	t.Setenv("ANTHROPIC_API_KEY", "ak-test")
	t.Setenv("ANTHROPIC_BASE_URL", srv.URL+"/v1")

	resp, desc, err := Chat(context.Background(), "anthropic", "", Request{
		System:      "sys",
		Messages:    []Message{{Role: "user", Content: "hi"}},
		Temperature: 0,
	})
	if err != nil {
		t.Fatalf("Chat 失败: %v", err)
	}
	if desc.Kind != "anthropic" {
		t.Fatalf("kind 不对: %s", desc.Kind)
	}
	if gotKey != "ak-test" || gotVersion == "" {
		t.Errorf("Anthropic 头不对: key=%q version=%q", gotKey, gotVersion)
	}
	if gotBody["system"] != "sys" {
		t.Errorf("system 字段应提升到顶层: %v", gotBody["system"])
	}
	if gotBody["max_tokens"] == nil {
		t.Errorf("Anthropic 必须带 max_tokens")
	}
	if resp.Text != "结论：通过" {
		t.Errorf("content 数组解析错误: %q", resp.Text)
	}
}

func TestGeminiChat(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path + "?" + r.URL.RawQuery
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, "{\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"OK\"}]}}],\"usageMetadata\":{\"totalTokenCount\":18}}")
	}))
	defer srv.Close()

	t.Setenv("GEMINI_API_KEY", "gk-test")
	t.Setenv("GEMINI_BASE_URL", srv.URL+"/v1beta")

	resp, desc, err := Chat(context.Background(), "gemini", "gemini-2.0-flash", Request{
		Messages:    []Message{{Role: "user", Content: "hi"}},
		Temperature: 0,
	})
	if err != nil {
		t.Fatalf("Chat 失败: %v", err)
	}
	if desc.Kind != "gemini" {
		t.Fatalf("kind 不对: %s", desc.Kind)
	}
	if !strings.Contains(gotPath, ":generateContent") || !strings.Contains(gotPath, "key=gk-test") {
		t.Errorf("Gemini 路径/key 不对: %q", gotPath)
	}
	if gotBody["contents"] == nil {
		t.Errorf("contents 缺失")
	}
	if resp.Text != "OK" {
		t.Errorf("candidates 解析错误: %q", resp.Text)
	}
}

func TestUnknownProvider(t *testing.T) {
	if _, _, err := Chat(context.Background(), "no-such-provider", "x", Request{}); err == nil {
		t.Fatal("未知 provider 应当报错")
	}
}

func TestLocalProviderNeedsNoKey(t *testing.T) {
	for _, name := range []string{"ollama", "vllm", "lmstudio"} {
		p, ok := Resolve(name)
		if !ok {
			t.Fatalf("%s 未注册", name)
		}
		if p.KeyEnv != "" {
			t.Errorf("%s 是本地推理，不应要求密钥", name)
		}
		if !p.KeyPresent() {
			t.Errorf("%s 的 KeyPresent 应为 true（无需密钥）", name)
		}
	}
}

func TestProfileCount(t *testing.T) {
	if n := len(Profiles()); n < 20 {
		t.Fatalf("内置 profile 数量异常: %d", n)
	}
}
