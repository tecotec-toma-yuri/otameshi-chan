package llm

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/otameshi/backend/internal/config"
	"github.com/otameshi/backend/internal/external/httpclient"
)

type ChatMessage struct {
	Role       string         `json:"role"`
	Content    string         `json:"content,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
	ToolCalls  []ChatToolCall `json:"tool_calls,omitempty"`
}

type ChatToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function ChatFunctionCall `json:"function"`
}

type ChatFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Tools       []chatTool    `json:"tools,omitempty"`
	Stream      bool          `json:"stream"`
	Temperature float64       `json:"temperature,omitempty"`
}

type chatTool struct {
	Type     string           `json:"type"`
	Function chatToolFunction `json:"function"`
}

type chatToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type streamChunk struct {
	Choices []streamChoice `json:"choices"`
}

type streamChoice struct {
	Delta        streamDelta `json:"delta"`
	FinishReason *string     `json:"finish_reason"`
}

type streamDelta struct {
	Content   string           `json:"content,omitempty"`
	ToolCalls []streamToolCall `json:"tool_calls,omitempty"`
}

type streamToolCall struct {
	Index    int          `json:"index"`
	ID       string       `json:"id,omitempty"`
	Type     string       `json:"type,omitempty"`
	Function streamFnCall `json:"function,omitempty"`
}

type streamFnCall struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type ChatCompletionClient struct {
	httpclient.BaseClient
	mu            sync.Mutex
	apiKey        string
	model         string
	history       []ChatMessage
	tools         []chatTool
	toolsDisabled bool
}

func NewChatCompletionClient() *ChatCompletionClient {
	cfg := config.Get()
	infra := config.Infra()
	apiKey := infra.OpenAIAPIKey
	model := cfg.LLMModel
	if model == "" {
		model = "gpt-4o-mini"
	}
	baseURL := infra.OpenAIBaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	systemPrompt := buildSystemPrompt(cfg)

	c := &ChatCompletionClient{
		BaseClient: httpclient.NewBaseClient(baseURL, 120*time.Second),
		apiKey:     apiKey,
		model:      model,
		tools:      buildTools(cfg.Products),
		history: []ChatMessage{
			{Role: "system", Content: systemPrompt},
		},
	}
	return c
}

func (c *ChatCompletionClient) SendUserMessage(ctx context.Context, text string) (<-chan StreamEvent, error) {
	c.mu.Lock()
	c.history = append(c.history, ChatMessage{Role: "user", Content: text})
	messages := make([]ChatMessage, len(c.history))
	copy(messages, c.history)
	c.mu.Unlock()

	ch := make(chan StreamEvent, 64)

	go func() {
		defer close(ch)
		c.doStreamRequest(ctx, messages, ch, true)
	}()

	return ch, nil
}

// RequestGreeting asks the LLM to produce a short opening greeting.
// The internal instruction is not stored as a user turn; only the assistant reply is kept in history.
func (c *ChatCompletionClient) RequestGreeting(ctx context.Context) (<-chan StreamEvent, error) {
	const instruction = "お客様が接続しました。おためしちゃんとして、1〜2文で明るく短く挨拶してください。商品の推薦や関数呼び出しはしないでください。"

	c.mu.Lock()
	messages := make([]ChatMessage, len(c.history), len(c.history)+1)
	copy(messages, c.history)
	messages = append(messages, ChatMessage{Role: "user", Content: instruction})
	c.mu.Unlock()

	ch := make(chan StreamEvent, 64)

	go func() {
		defer close(ch)
		c.doStreamRequest(ctx, messages, ch, false)
	}()

	return ch, nil
}

func (c *ChatCompletionClient) AppendAssistantMessage(text string) {
	if text == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.history = append(c.history, ChatMessage{Role: "assistant", Content: text})
}

func (c *ChatCompletionClient) RestoreHistory(messages []ChatMessage) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.history = append(c.history[:1], messages...)
}

func (c *ChatCompletionClient) History() []ChatMessage {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]ChatMessage, len(c.history))
	copy(out, c.history)
	return out
}

func (c *ChatCompletionClient) Close() error {
	return nil
}

func buildTools(products []config.Product) []chatTool {
	result := []chatTool{
		{
			Type: "function",
			Function: chatToolFunction{
				Name:        "end_conversation",
				Description: "会話を終了する。お客様が会話を終えたい意思を示した時に使う。",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"closing_speech": {
							"type": "string",
							"description": "終了時に読み上げるお別れのセリフ"
						}
					},
					"required": ["closing_speech"]
				}`),
			},
		},
	}

	if len(products) > 0 {
		productIDs := make([]string, 0, len(products))
		for _, p := range products {
			productIDs = append(productIDs, fmt.Sprintf("%q", p.ID))
		}
		enumJSON := "[" + strings.Join(productIDs, ",") + "]"

		result = append([]chatTool{
			{
				Type: "function",
				Function: chatToolFunction{
					Name:        "recommend_product",
					Description: "お客様が商品について具体的に聞いた時や、おすすめを求めた時にのみ呼び出す。雑談や挨拶では呼び出さないこと。",
					Parameters: json.RawMessage(fmt.Sprintf(`{
						"type": "object",
						"properties": {
							"product_ids": {
								"type": "array",
								"items": {"type": "string", "enum": %s},
								"description": "おすすめする商品IDの配列"
							},
							"introduction_speech": {
								"type": "string",
								"description": "商品を紹介するときの発話テキスト"
							}
						},
						"required": ["product_ids", "introduction_speech"]
					}`, enumJSON)),
				},
			},
		}, result...)
	}

	return result
}

func buildSystemPrompt(cfg config.Config) string {
	prompt := cfg.SystemPrompt
	if len(cfg.Products) == 0 {
		return prompt
	}

	var sb strings.Builder
	sb.WriteString(prompt)
	sb.WriteString("\n\n## 取扱商品一覧\n")
	for _, p := range cfg.Products {
		sb.WriteString(fmt.Sprintf("- %s: %s（%d円）— %s\n", p.ID, p.Name, p.Price, p.Description))
	}
	sb.WriteString("\n## 関数の使い方\n")
	sb.WriteString("- recommend_product: お客様が商品について具体的に質問したり、「おすすめを教えて」「何がある？」など明確に商品紹介を求めた場合にのみ使う。雑談や挨拶では絶対に使わないこと。\n")
	sb.WriteString("- end_conversation: お客様が「さようなら」「ありがとう、もう大丈夫」など会話を終えたい意思を明確に示した時に使う。\n")
	return sb.String()
}

func (c *ChatCompletionClient) doHTTPRequest(ctx context.Context, reqBody *chatRequest) (*http.Response, error) {
	return c.PostJSON(ctx, "/chat/completions", reqBody, c.apiKey)
}

func (c *ChatCompletionClient) doStreamRequest(ctx context.Context, messages []ChatMessage, ch chan<- StreamEvent, includeTools bool) {
	started := time.Now()
	model := config.Get().LLMModel
	if model == "" {
		model = c.model
	}

	c.mu.Lock()
	disabled := c.toolsDisabled
	c.mu.Unlock()

	var reqTools []chatTool
	if includeTools && !disabled {
		reqTools = c.tools
	}

	reqBody := chatRequest{
		Model:    model,
		Messages: messages,
		Tools:    reqTools,
		Stream:   true,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		slog.Error("failed to marshal chat request", "error", err)
		return
	}

	slog.Info("LLM request",
		"url", c.BaseURL+"/chat/completions",
		"model", model,
		"stream", true,
		"message_count", len(messages),
		"tool_count", len(reqTools),
		"params", string(body),
	)

	resp, err := c.doHTTPRequest(ctx, &reqBody)
	if err != nil {
		slog.Error("chat completion request failed", "error", err, "duration_ms", time.Since(started).Milliseconds())
		return
	}
	defer resp.Body.Close()

	headersMs := time.Since(started).Milliseconds()
	slog.Info("latency", "stage", "llm_headers", "duration_ms", headersMs)

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		slog.Error("chat completion error", "status", resp.StatusCode, "body", string(respBody))
		if resp.StatusCode == 400 && len(reqTools) > 0 && strings.Contains(string(respBody), "tool") {
			slog.Warn("retrying without tools (model may not support function calling)")
			c.mu.Lock()
			c.toolsDisabled = true
			c.mu.Unlock()
			reqBody.Tools = nil
			resp.Body.Close()
			resp2, err2 := c.doHTTPRequest(ctx, &reqBody)
			if err2 != nil {
				slog.Error("retry without tools failed", "error", err2)
				return
			}
			resp.Body = resp2.Body
			started = time.Now()
			if resp2.StatusCode != http.StatusOK {
				b, _ := io.ReadAll(resp2.Body)
				resp2.Body.Close()
				slog.Error("retry still failed", "status", resp2.StatusCode, "body", string(b))
				return
			}
		} else {
			return
		}
	}

	var fullText string
	var firstTokenAt time.Time
	var textDoneSent bool
	toolCalls := make(map[int]*struct {
		id   string
		name string
		args strings.Builder
	})

	scanner := bufio.NewScanner(resp.Body)
	// Function call arguments can be large
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var raw map[string]json.RawMessage
		if err := json.Unmarshal([]byte(data), &raw); err != nil {
			continue
		}
		if _, hasErr := raw["error"]; hasErr {
			slog.Error("SSE stream error", "data", data)
			select {
			case <-ctx.Done():
			case ch <- StreamEvent{Type: "error", Text: data}:
			}
			return
		}

		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		choice := chunk.Choices[0]
		delta := choice.Delta

		if delta.Content != "" {
			if firstTokenAt.IsZero() {
				firstTokenAt = time.Now()
				slog.Info("latency", "stage", "llm_ttfb", "duration_ms", time.Since(started).Milliseconds())
			}
			fullText += delta.Content
			select {
			case <-ctx.Done():
				return
			case ch <- StreamEvent{Type: "text_delta", Text: delta.Content}:
			}
		}

		for _, tc := range delta.ToolCalls {
			if firstTokenAt.IsZero() {
				firstTokenAt = time.Now()
				slog.Info("latency", "stage", "llm_ttfb", "duration_ms", time.Since(started).Milliseconds())
			}
			entry, ok := toolCalls[tc.Index]
			if !ok {
				entry = &struct {
					id   string
					name string
					args strings.Builder
				}{}
				toolCalls[tc.Index] = entry
			}
			if tc.ID != "" {
				entry.id = tc.ID
			}
			if tc.Function.Name != "" {
				entry.name = tc.Function.Name
			}
			if tc.Function.Arguments != "" {
				entry.args.WriteString(tc.Function.Arguments)
			}
		}

		if choice.FinishReason != nil {
			switch *choice.FinishReason {
			case "stop":
				if fullText != "" {
					c.mu.Lock()
					c.history = append(c.history, ChatMessage{Role: "assistant", Content: fullText})
					c.mu.Unlock()

					textDoneSent = true
					select {
					case <-ctx.Done():
						return
					case ch <- StreamEvent{Type: "text_done", Text: fullText}:
					}
				}

			case "tool_calls":
				for _, tc := range toolCalls {
					argsJSON := json.RawMessage(tc.args.String())
					speech := extractToolSpeech(tc.name, tc.args.String())

					if speech != "" {
						c.mu.Lock()
						c.history = append(c.history, ChatMessage{Role: "assistant", Content: speech})
						c.mu.Unlock()
					}

					select {
					case <-ctx.Done():
						return
					case ch <- StreamEvent{
						Type:         "function_call",
						FunctionName: tc.name,
						FunctionArgs: argsJSON,
					}:
					}
				}
			}
		}
	}

	if fullText != "" && !textDoneSent {
		slog.Warn("LLM stream ended without finish_reason, flushing text", "text_len", len(fullText))
		c.mu.Lock()
		c.history = append(c.history, ChatMessage{Role: "assistant", Content: fullText})
		c.mu.Unlock()
		select {
		case <-ctx.Done():
		case ch <- StreamEvent{Type: "text_done", Text: fullText}:
		}
	}

	slog.Info("latency", "stage", "llm_total", "duration_ms", time.Since(started).Milliseconds())
}

func extractToolSpeech(name, argsJSON string) string {
	switch name {
	case "end_conversation":
		var args EndConversationArgs
		if err := json.Unmarshal([]byte(argsJSON), &args); err == nil {
			return args.ClosingSpeech
		}
	case "recommend_product":
		var args RecommendProductArgs
		if err := json.Unmarshal([]byte(argsJSON), &args); err == nil {
			return args.IntroductionSpeech
		}
	}
	return ""
}
