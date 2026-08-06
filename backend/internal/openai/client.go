package openai

import (
	"context"
	"encoding/json"

	"github.com/otameshi/backend/internal/config"
)

// StreamEvent represents a single event from the realtime API stream.
type StreamEvent struct {
	Type         string          // "text_delta", "text_done", "function_call"
	Text         string
	FunctionName string          // "recommend_product" or "end_conversation"
	FunctionArgs json.RawMessage
}

// RealtimeClient abstracts the OpenAI Realtime API.
type RealtimeClient interface {
	SendUserMessage(ctx context.Context, text string) (<-chan StreamEvent, error)
	RequestGreeting(ctx context.Context) (<-chan StreamEvent, error)
	RequestRecommendation(ctx context.Context) (<-chan StreamEvent, error)
	RequestSequentialRecommendation(ctx context.Context, productID string) (<-chan StreamEvent, error)
	AppendAssistantMessage(text string)
	UpdateSystemPrompt(prompt string)
	InjectProductCatalog(catalogText string)
	SetRecommendationTools(products []config.Product)
	RestoreHistory(messages []ChatMessage)
	History() []ChatMessage
	Close() error
}
