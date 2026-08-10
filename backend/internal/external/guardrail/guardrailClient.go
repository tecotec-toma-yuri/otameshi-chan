package guardrail

import (
	"context"
	"encoding/json"
	"time"

	"github.com/otameshi/backend/internal/config"
	"github.com/otameshi/backend/internal/external/httpclient"
)

// LLMGuardrailMonitor uses an LLM API to check content policy.
type LLMGuardrailMonitor struct {
	httpclient.BaseClient
	apiKey string
	model  string
}

func NewGuardrailMonitor() *LLMGuardrailMonitor {
	infra := config.Infra()
	baseURL := infra.OpenAIBaseURL
	if baseURL == "" {
		baseURL = "https://api.groq.com/openai/v1"
	}
	return &LLMGuardrailMonitor{
		BaseClient: httpclient.NewBaseClient(baseURL, 5*time.Second),
		apiKey:     infra.OpenAIAPIKey,
		model:      "llama-3.1-8b-instant",
	}
}

type moderationRequest struct {
	Model    string              `json:"model"`
	Messages []moderationMessage `json:"messages"`
	Stream   bool                `json:"stream"`
}

type moderationMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type moderationResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type moderationResult struct {
	Flagged  bool   `json:"flagged"`
	Category string `json:"category"`
}

const moderationPrompt = `You are a content moderation system. Analyze the following user text and determine if it violates content policies.

Flag the text if it contains:
- Hate speech, discrimination, or slurs
- Explicit sexual content
- Threats of violence or self-harm
- Personally identifiable information shared maliciously
- Illegal activity instructions

Respond ONLY with a JSON object: {"flagged": false} or {"flagged": true, "category": "<category>"}
Do not include any other text.`

func (m *LLMGuardrailMonitor) Check(ctx context.Context, text string) (bool, string, error) {
	if m.apiKey == "" {
		return true, "", nil
	}

	resp, err := m.PostJSON(ctx, "/chat/completions", moderationRequest{
		Model: m.model,
		Messages: []moderationMessage{
			{Role: "system", Content: moderationPrompt},
			{Role: "user", Content: text},
		},
		Stream: false,
	}, m.apiKey)
	if err != nil {
		return true, "", err
	}

	var modResp moderationResponse
	if err := httpclient.DecodeJSON(resp, &modResp); err != nil {
		return true, "", err
	}

	if len(modResp.Choices) == 0 {
		return true, "", nil
	}

	content := modResp.Choices[0].Message.Content
	var result moderationResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return true, "", nil
	}

	if result.Flagged {
		return false, result.Category, nil
	}
	return true, "", nil
}
