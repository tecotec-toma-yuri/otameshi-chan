package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/otameshi/backend/internal/config"
)

// GuardrailMonitor checks text for content policy violations.
type GuardrailMonitor interface {
	Check(ctx context.Context, text string) (passed bool, reason string, err error)
}

// LLMGuardrailMonitor uses an LLM API to check content policy.
type LLMGuardrailMonitor struct {
	baseURL string
	apiKey  string
	model   string
	client  *http.Client
}

func NewGuardrailMonitor() *LLMGuardrailMonitor {
	cfg := config.Get()
	baseURL := cfg.OpenAIBaseURL
	if baseURL == "" {
		baseURL = "https://api.groq.com/openai/v1"
	}
	return &LLMGuardrailMonitor{
		baseURL: baseURL,
		apiKey:  cfg.OpenAIAPIKey,
		model:   "llama-3.1-8b-instant",
		client:  &http.Client{Timeout: 5 * time.Second},
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

	body, err := json.Marshal(moderationRequest{
		Model: m.model,
		Messages: []moderationMessage{
			{Role: "system", Content: moderationPrompt},
			{Role: "user", Content: text},
		},
		Stream: false,
	})
	if err != nil {
		return true, "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return true, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+m.apiKey)

	resp, err := m.client.Do(req)
	if err != nil {
		return true, "", fmt.Errorf("moderation request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body)
		return true, "", fmt.Errorf("moderation API returned %d", resp.StatusCode)
	}

	var modResp moderationResponse
	if err := json.NewDecoder(resp.Body).Decode(&modResp); err != nil {
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

// StubGuardrailMonitor is a guardrail monitor that always passes, except for test triggers.
type StubGuardrailMonitor struct{}

// NewStubGuardrailMonitor creates a new StubGuardrailMonitor.
func NewStubGuardrailMonitor() *StubGuardrailMonitor {
	return &StubGuardrailMonitor{}
}

// Check performs a content policy check. Always passes unless text contains "violation_test".
func (m *StubGuardrailMonitor) Check(_ context.Context, text string) (bool, string, error) {
	time.Sleep(50 * time.Millisecond)

	if strings.Contains(text, "violation_test") {
		return false, "content_policy_violation", nil
	}
	return true, "", nil
}
