package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/otameshi/backend/internal/config"
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
	Model       string          `json:"model"`
	Messages    []ChatMessage   `json:"messages"`
	Tools       []chatTool      `json:"tools,omitempty"`
	ToolChoice  json.RawMessage `json:"tool_choice,omitempty"`
	Stream      bool            `json:"stream"`
	Temperature float64         `json:"temperature,omitempty"`
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
	mu            sync.Mutex
	apiKey        string
	model         string
	baseURL       string
	history       []ChatMessage
	tools         []chatTool
	toolsDisabled bool
	client        *http.Client
}

func NewChatCompletionClient() *ChatCompletionClient {
	apiKey := os.Getenv("OPENAI_API_KEY")
	cfg := config.Get()
	model := cfg.LLMModel
	if model == "" {
		model = "gpt-4o-mini"
	}
	baseURL := os.Getenv("OPENAI_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	systemPrompt := buildStage1SystemPrompt(cfg)

	c := &ChatCompletionClient{
		apiKey:  apiKey,
		model:   model,
		baseURL: baseURL,
		client:  &http.Client{Timeout: 120 * time.Second},
		tools:   buildStage1Tools(cfg),
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

	cfg := config.Get()
	c.mu.Lock()
	// Use base system prompt without interest guidelines for greeting
	greetingMessages := []ChatMessage{
		{Role: "system", Content: cfg.SystemPrompt},
		{Role: "user", Content: instruction},
	}
	c.mu.Unlock()

	ch := make(chan StreamEvent, 64)

	go func() {
		defer close(ch)
		c.doStreamRequest(ctx, greetingMessages, ch, false)
	}()

	return ch, nil
}

func (c *ChatCompletionClient) RequestRecommendation(ctx context.Context) (<-chan StreamEvent, error) {
	c.mu.Lock()
	messages := make([]ChatMessage, len(c.history))
	copy(messages, c.history)
	c.mu.Unlock()

	ch := make(chan StreamEvent, 64)
	go func() {
		defer close(ch)
		c.doStreamRequestForced(ctx, messages, ch, "recommend_product")
	}()
	return ch, nil
}

func (c *ChatCompletionClient) RequestSequentialRecommendation(ctx context.Context, productID string) (<-chan StreamEvent, error) {
	c.mu.Lock()
	c.tools = buildSequentialRecommendTool(productID)
	messages := make([]ChatMessage, len(c.history))
	copy(messages, c.history)
	c.mu.Unlock()

	ch := make(chan StreamEvent, 64)
	go func() {
		defer close(ch)
		c.doStreamRequestForced(ctx, messages, ch, "recommend_product")
	}()
	return ch, nil
}

func (c *ChatCompletionClient) UpdateSystemPrompt(prompt string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.history) > 0 && c.history[0].Role == "system" {
		c.history[0].Content = prompt
	}
}

func (c *ChatCompletionClient) InjectProductCatalog(catalogText string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.history) > 0 && c.history[0].Role == "system" {
		c.history[0].Content += "\n\n" + catalogText
	}
}

func (c *ChatCompletionClient) SetRecommendationTools(products []config.Product) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.tools = buildRecommendTools(products)
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

// ---------------------------------------------------------------------------
// Stage 1 tools: assess_interest + end_conversation
// ---------------------------------------------------------------------------

func buildStage1Tools(cfg config.Config) []chatTool {
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

	if len(cfg.Products) > 0 {
		result = append([]chatTool{
			{
				Type: "function",
				Function: chatToolFunction{
					Name:        "assess_interest",
					Description: "会話の応答と同時に、お客様の商品への興味度を判定する。毎回の会話で呼び出し、興味の度合いを1〜5で返す。応答テキストとは別に、この関数も同時に呼ぶこと。",
					Parameters: json.RawMessage(`{
						"type": "object",
						"properties": {
							"interest_level": {
								"type": "integer",
								"minimum": 1,
								"maximum": 5,
								"description": "興味の度合い（1=興味なし 2=わずかに関連 3=やや興味あり 4=明確な興味 5=強い購買意欲）"
							},
							"detected_preferences": {
								"type": "array",
								"items": {"type": "string"},
								"description": "会話から検出したユーザーの好み（例: [\"甘い\", \"フルーティー\"]）。興味が低い場合は空配列"
							},
							"response_text": {
								"type": "string",
								"description": "興味度が高い場合にユーザーに返す反応（例: 「甘いものがお好きなんですね！」）。興味が低い場合は空文字"
							},
							"trigger_utterance": {
								"type": "string",
								"description": "判定の根拠となったユーザーの発言の要約"
							}
						},
						"required": ["interest_level", "detected_preferences", "response_text", "trigger_utterance"]
					}`),
				},
			},
		}, result...)
	}

	return result
}

// ---------------------------------------------------------------------------
// Stage 2 tools: recommend_product (full catalog)
// ---------------------------------------------------------------------------

func buildRecommendTools(products []config.Product) []chatTool {
	productIDs := make([]string, 0, len(products))
	for _, p := range products {
		productIDs = append(productIDs, fmt.Sprintf("%q", p.ID))
	}
	enumJSON := "[" + strings.Join(productIDs, ",") + "]"

	return []chatTool{
		{
			Type: "function",
			Function: chatToolFunction{
				Name:        "recommend_product",
				Description: "お客様の好みに最も合う商品を1つ選び紹介する。",
				Parameters: json.RawMessage(fmt.Sprintf(`{
					"type": "object",
					"properties": {
						"product_id": {
							"type": "string",
							"enum": %s,
							"description": "おすすめする商品ID"
						},
						"introduction_speech": {
							"type": "string",
							"description": "商品を紹介するときの発話テキスト"
						},
						"reason": {
							"type": "string",
							"description": "この商品を薦める理由"
						}
					},
					"required": ["product_id", "introduction_speech", "reason"]
				}`, enumJSON)),
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Sequential recommend tool: single product for related-product flow
// ---------------------------------------------------------------------------

func buildSequentialRecommendTool(productID string) []chatTool {
	return []chatTool{
		{
			Type: "function",
			Function: chatToolFunction{
				Name:        "recommend_product",
				Description: "関連商品をお客様に紹介する。先に紹介した商品との関連性に触れながら自然につなげること。",
				Parameters: json.RawMessage(fmt.Sprintf(`{
					"type": "object",
					"properties": {
						"product_id": {
							"type": "string",
							"enum": [%q],
							"description": "おすすめする商品ID"
						},
						"introduction_speech": {
							"type": "string",
							"description": "商品を紹介するときの発話テキスト"
						},
						"reason": {
							"type": "string",
							"description": "この商品を薦める理由"
						}
					},
					"required": ["product_id", "introduction_speech", "reason"]
				}`, productID)),
			},
		},
	}
}

// ---------------------------------------------------------------------------
// System prompts
// ---------------------------------------------------------------------------

func buildStage1SystemPrompt(cfg config.Config) string {
	prompt := cfg.SystemPrompt
	if len(cfg.Products) == 0 {
		return prompt
	}

	var sb strings.Builder
	sb.WriteString(prompt)
	sb.WriteString("\n\n## 興味判定ガイドライン\n\n")
	sb.WriteString("あなたは通常の会話応答と同時に、お客様の商品への興味度を判定します。\n")
	sb.WriteString("毎回の応答で assess_interest 関数を呼び出し、興味度を1〜5で判定してください。\n\n")
	sb.WriteString("### 判定基準\n\n")
	sb.WriteString(cfg.InterestPrompt)
	sb.WriteString("\n\n### 興味度の5段階\n")
	sb.WriteString("- 1: 興味なし（上記の判定基準に該当しない）\n")
	sb.WriteString("- 2: わずかに関連（判定基準にかすかに触れる程度）\n")
	sb.WriteString("- 3: やや興味あり（判定基準に該当する話題が出ている）\n")
	sb.WriteString("- 4: 明確な興味（直接的に商品やおすすめを求めている）\n")
	sb.WriteString("- 5: 強い購買意欲（買いたい・試したい等の意欲を示している）\n\n")
	sb.WriteString("### response_text の書き方\n")
	sb.WriteString("- 興味度3以上の場合: お客様の興味に反応する一言を返す\n")
	sb.WriteString("  （「甘いものがお好きなんですね！」「お菓子に興味がおありですか？」等）\n")
	sb.WriteString("- 興味度1〜2の場合: 空文字を返す\n\n")
	sb.WriteString("## 関数の使い方\n")
	sb.WriteString("- assess_interest: 毎回の応答で呼び出す。通常の会話応答テキストとは別に、この関数も同時に呼ぶこと。\n")
	sb.WriteString("- end_conversation: お客様が「さようなら」「ありがとう、もう大丈夫」など会話を終えたい意思を明確に示した時に使う。\n")
	return sb.String()
}

// BuildProductCatalogPrompt returns the text to inject into the system prompt
// when transitioning to stage 2 (recommendation).
func BuildProductCatalogPrompt(products []config.Product) string {
	var sb strings.Builder
	sb.WriteString("## 取扱商品一覧\n")
	for _, p := range products {
		if len(p.Tags) > 0 {
			sb.WriteString(fmt.Sprintf("- %s: %s [%s]\n  %s\n", p.ID, p.Name, strings.Join(p.Tags, ","), p.Description))
		} else {
			sb.WriteString(fmt.Sprintf("- %s: %s\n  %s\n", p.ID, p.Name, p.Description))
		}
	}
	sb.WriteString("\n## 商品推薦ガイドライン\n")
	sb.WriteString("お客様の好みに最も合う商品を1つ選び、recommend_product 関数で紹介してください。\n")
	sb.WriteString("- 必ず1商品を選ぶこと\n")
	sb.WriteString("- なぜその商品を薦めるのか理由を添える（「甘いのがお好きとのことでしたので」等）\n")
	sb.WriteString("- introduction_speech は2文以内で簡潔に\n")
	return sb.String()
}

// ---------------------------------------------------------------------------
// HTTP + SSE streaming
// ---------------------------------------------------------------------------

func (c *ChatCompletionClient) doHTTPRequest(ctx context.Context, reqBody *chatRequest) (*http.Response, error) {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	return c.client.Do(req)
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
		"url", c.baseURL+"/chat/completions",
		"model", model,
		"stream", true,
		"message_count", len(messages),
		"tool_count", len(reqTools),
		"params", string(body),
	)

	c.streamAndParse(ctx, &reqBody, ch, started, reqTools)
}

func (c *ChatCompletionClient) doStreamRequestForced(ctx context.Context, messages []ChatMessage, ch chan<- StreamEvent, forceFn string) {
	started := time.Now()
	model := config.Get().LLMModel
	if model == "" {
		model = c.model
	}

	c.mu.Lock()
	tools := make([]chatTool, len(c.tools))
	copy(tools, c.tools)
	c.mu.Unlock()

	toolChoice, _ := json.Marshal(map[string]interface{}{
		"type": "function",
		"function": map[string]string{
			"name": forceFn,
		},
	})

	reqBody := chatRequest{
		Model:      model,
		Messages:   messages,
		Tools:      tools,
		ToolChoice: toolChoice,
		Stream:     true,
	}

	slog.Info("LLM request (forced)",
		"url", c.baseURL+"/chat/completions",
		"model", model,
		"stream", true,
		"message_count", len(messages),
		"tool_count", len(tools),
		"forced_function", forceFn,
	)

	c.streamAndParse(ctx, &reqBody, ch, started, tools)
}

func (c *ChatCompletionClient) streamAndParse(ctx context.Context, reqBody *chatRequest, ch chan<- StreamEvent, started time.Time, reqTools []chatTool) {
	resp, err := c.doHTTPRequest(ctx, reqBody)
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
			reqBody.ToolChoice = nil
			resp.Body.Close()
			resp2, err2 := c.doHTTPRequest(ctx, reqBody)
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
				// Some models dump function call markup into content — strip it.
				// Pass cleaned text along with the first function_call event
				// so the session manager can finalize the text bubble.
				cleanedText := stripFunctionCallMarkup(fullText)
				historyAdded := false
				if cleanedText != "" && !textDoneSent {
					c.mu.Lock()
					c.history = append(c.history, ChatMessage{Role: "assistant", Content: cleanedText})
					c.mu.Unlock()
					historyAdded = true
				}

				first := true
				for _, tc := range toolCalls {
					argsJSON := json.RawMessage(tc.args.String())
					speech := extractToolSpeech(tc.name, tc.args.String())

					if speech != "" {
						c.mu.Lock()
						c.history = append(c.history, ChatMessage{Role: "assistant", Content: speech})
						c.mu.Unlock()
						historyAdded = true
					}

					ev := StreamEvent{
						Type:         "function_call",
						FunctionName: tc.name,
						FunctionArgs: argsJSON,
					}
					if first && cleanedText != "" && !textDoneSent {
						ev.Text = cleanedText
						first = false
					}

					select {
					case <-ctx.Done():
						return
					case ch <- ev:
					}
				}

				// Ensure an assistant turn exists in history to prevent consecutive user messages.
				// This happens when the model emits a tool call with no text content and no speech
				// (e.g. assess_interest with response_text="" for low-interest turns).
				if !historyAdded && !textDoneSent {
					c.mu.Lock()
					c.history = append(c.history, ChatMessage{Role: "assistant", Content: "（処理中）"})
					c.mu.Unlock()
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

// stripFunctionCallMarkup removes function-call metadata that some models
// erroneously include in the text content alongside proper tool_calls.
// Common patterns: "**Response:**\n...\n**Function call:**\n{...}"
func stripFunctionCallMarkup(text string) string {
	markers := []string{
		"**Function call:**",
		"**Function Call:**",
		"**function_call:**",
		"\n```json\n{",
	}
	cleaned := text
	for _, marker := range markers {
		if idx := strings.Index(cleaned, marker); idx >= 0 {
			cleaned = cleaned[:idx]
		}
	}
	// Also strip "**Response:**" header prefix
	cleaned = strings.TrimPrefix(cleaned, "**Response:**")
	cleaned = strings.TrimPrefix(cleaned, "**Response:** ")
	cleaned = strings.TrimSpace(cleaned)
	return cleaned
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
	case "assess_interest":
		var args AssessInterestArgs
		if err := json.Unmarshal([]byte(argsJSON), &args); err == nil {
			return args.ResponseText
		}
	}
	return ""
}
