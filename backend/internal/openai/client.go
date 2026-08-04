package openai

import (
	"context"
	"encoding/json"
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
	AppendAssistantMessage(text string)
	RestoreHistory(messages []ChatMessage)
	History() []ChatMessage
	Close() error
}
