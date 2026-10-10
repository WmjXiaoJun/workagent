package llm

// 主流模型适配层 —— 零第三方依赖，只用标准库。
//
// 设计:
//  1. 绝大多数厂商提供 OpenAI 兼容端点，因此用「一个引擎 + 一张 profile 表」覆盖。
//  2. 少数原生协议（Anthropic / Gemini）单独实现。
//  3. 本地推理（Ollama / vLLM / LM Studio）无需 API Key，因此天然适合做可复现验证。

import (
	"os"
	"strings"
)

// Profile 描述一个可用的模型服务商。
type Profile struct {
	Name         string
	Kind         string // openai | anthropic | gemini
	BaseURL      string
	BaseURLEnv   string // 允许用户覆盖端点
	KeyEnv       string // 空表示无需密钥（本地推理）
	DefaultModel string
	Vendor       string
	Note         string
}

var profiles = []Profile{
	// ---- 本地推理：无需密钥，可离线、可复现 ----
	{Name: "ollama", Kind: "openai", BaseURL: "http://127.0.0.1:11434/v1", BaseURLEnv: "OLLAMA_BASE_URL", KeyEnv: "", DefaultModel: "qwen2.5:7b", Vendor: "Ollama", Note: "本地开源权重，可做到端到端可复现"},
	{Name: "vllm", Kind: "openai", BaseURL: "http://127.0.0.1:8000/v1", BaseURLEnv: "VLLM_BASE_URL", KeyEnv: "", DefaultModel: "", Vendor: "vLLM", Note: "本地部署，可固定引擎构建"},
	{Name: "lmstudio", Kind: "openai", BaseURL: "http://127.0.0.1:1234/v1", BaseURLEnv: "LMSTUDIO_BASE_URL", KeyEnv: "", DefaultModel: "", Vendor: "LM Studio", Note: "本地桌面推理"},
	{Name: "custom", Kind: "openai", BaseURL: "", BaseURLEnv: "CUSTOM_BASE_URL", KeyEnv: "CUSTOM_API_KEY", DefaultModel: "", Vendor: "自定义", Note: "任何 OpenAI 兼容端点"},

	// ---- 国际 ----
	{Name: "openai", Kind: "openai", BaseURL: "https://api.openai.com/v1", BaseURLEnv: "OPENAI_BASE_URL", KeyEnv: "OPENAI_API_KEY", DefaultModel: "gpt-4o-mini", Vendor: "OpenAI"},
	{Name: "anthropic", Kind: "anthropic", BaseURL: "https://api.anthropic.com/v1", BaseURLEnv: "ANTHROPIC_BASE_URL", KeyEnv: "ANTHROPIC_API_KEY", DefaultModel: "claude-sonnet-4-5", Vendor: "Anthropic"},
	{Name: "gemini", Kind: "gemini", BaseURL: "https://generativelanguage.googleapis.com/v1beta", BaseURLEnv: "GEMINI_BASE_URL", KeyEnv: "GEMINI_API_KEY", DefaultModel: "gemini-2.0-flash", Vendor: "Google"},
	{Name: "mistral", Kind: "openai", BaseURL: "https://api.mistral.ai/v1", BaseURLEnv: "MISTRAL_BASE_URL", KeyEnv: "MISTRAL_API_KEY", DefaultModel: "mistral-large-latest", Vendor: "Mistral"},
	{Name: "xai", Kind: "openai", BaseURL: "https://api.x.ai/v1", BaseURLEnv: "XAI_BASE_URL", KeyEnv: "XAI_API_KEY", DefaultModel: "grok-2-latest", Vendor: "xAI"},
	{Name: "groq", Kind: "openai", BaseURL: "https://api.groq.com/openai/v1", BaseURLEnv: "GROQ_BASE_URL", KeyEnv: "GROQ_API_KEY", DefaultModel: "llama-3.3-70b-versatile", Vendor: "Groq"},
	{Name: "together", Kind: "openai", BaseURL: "https://api.together.xyz/v1", BaseURLEnv: "TOGETHER_BASE_URL", KeyEnv: "TOGETHER_API_KEY", DefaultModel: "meta-llama/Llama-3.3-70B-Instruct-Turbo", Vendor: "Together"},
	{Name: "openrouter", Kind: "openai", BaseURL: "https://openrouter.ai/api/v1", BaseURLEnv: "OPENROUTER_BASE_URL", KeyEnv: "OPENROUTER_API_KEY", DefaultModel: "openai/gpt-4o-mini", Vendor: "OpenRouter", Note: "聚合数百个模型"},

	// ---- 国内 ----
	{Name: "deepseek", Kind: "openai", BaseURL: "https://api.deepseek.com/v1", BaseURLEnv: "DEEPSEEK_BASE_URL", KeyEnv: "DEEPSEEK_API_KEY", DefaultModel: "deepseek-chat", Vendor: "深度求索"},
	{Name: "qwen", Kind: "openai", BaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1", BaseURLEnv: "DASHSCOPE_BASE_URL", KeyEnv: "DASHSCOPE_API_KEY", DefaultModel: "qwen-plus", Vendor: "阿里通义"},
	{Name: "moonshot", Kind: "openai", BaseURL: "https://api.moonshot.cn/v1", BaseURLEnv: "MOONSHOT_BASE_URL", KeyEnv: "MOONSHOT_API_KEY", DefaultModel: "moonshot-v1-8k", Vendor: "月之暗面 Kimi"},
	{Name: "zhipu", Kind: "openai", BaseURL: "https://open.bigmodel.cn/api/paas/v4", BaseURLEnv: "ZHIPU_BASE_URL", KeyEnv: "ZHIPUAI_API_KEY", DefaultModel: "glm-4-plus", Vendor: "智谱 GLM"},
	{Name: "doubao", Kind: "openai", BaseURL: "https://ark.cn-beijing.volces.com/api/v3", BaseURLEnv: "ARK_BASE_URL", KeyEnv: "ARK_API_KEY", DefaultModel: "", Vendor: "字节豆包（火山方舟）", Note: "model 需填方舟推理接入点 ID"},
	{Name: "hunyuan", Kind: "openai", BaseURL: "https://api.hunyuan.cloud.tencent.com/v1", BaseURLEnv: "HUNYUAN_BASE_URL", KeyEnv: "HUNYUAN_API_KEY", DefaultModel: "hunyuan-turbo", Vendor: "腾讯混元"},
	{Name: "baichuan", Kind: "openai", BaseURL: "https://api.baichuan-ai.com/v1", BaseURLEnv: "BAICHUAN_BASE_URL", KeyEnv: "BAICHUAN_API_KEY", DefaultModel: "Baichuan4", Vendor: "百川"},
	{Name: "minimax", Kind: "openai", BaseURL: "https://api.minimax.chat/v1", BaseURLEnv: "MINIMAX_BASE_URL", KeyEnv: "MINIMAX_API_KEY", DefaultModel: "abab6.5s-chat", Vendor: "MiniMax"},
	{Name: "spark", Kind: "openai", BaseURL: "https://spark-api-open.xf-yun.com/v1", BaseURLEnv: "SPARK_BASE_URL", KeyEnv: "SPARK_API_KEY", DefaultModel: "generalv3.5", Vendor: "科大讯飞星火"},
	{Name: "siliconflow", Kind: "openai", BaseURL: "https://api.siliconflow.cn/v1", BaseURLEnv: "SILICONFLOW_BASE_URL", KeyEnv: "SILICONFLOW_API_KEY", DefaultModel: "Qwen/Qwen2.5-72B-Instruct", Vendor: "硅基流动"},
}

// Profiles 返回全部内置 profile。
func Profiles() []Profile { return profiles }

// Resolve 按名称查找 profile（大小写不敏感）。
func Resolve(name string) (Profile, bool) {
	n := strings.ToLower(strings.TrimSpace(name))
	for _, p := range profiles {
		if p.Name == n {
			return p, true
		}
	}
	return Profile{}, false
}

// Endpoint 返回实际使用的端点（环境变量优先）。
func (p Profile) Endpoint() string {
	if p.BaseURLEnv != "" {
		if v := strings.TrimSpace(os.Getenv(p.BaseURLEnv)); v != "" {
			return strings.TrimRight(v, "/")
		}
	}
	return strings.TrimRight(p.BaseURL, "/")
}

// APIKey 返回该 profile 的密钥（本地推理返回空）。
func (p Profile) APIKey() string {
	if p.KeyEnv == "" {
		return ""
	}
	return strings.TrimSpace(os.Getenv(p.KeyEnv))
}

// KeyPresent 报告密钥是否已配置；无需密钥的本地推理恒为 true。
func (p Profile) KeyPresent() bool {
	if p.KeyEnv == "" {
		return true
	}
	return p.APIKey() != ""
}
