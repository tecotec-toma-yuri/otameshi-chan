package factory

import (
	"log/slog"

	"github.com/otameshi/backend/internal/config"
	"github.com/otameshi/backend/internal/external/llm"
)

func NewLLMClient() llm.RealtimeClient {
	switch config.Infra().LLMMode {
	case "chat":
		slog.Info("using Chat Completion API")
		return llm.NewChatCompletionClient()
	default:
		slog.Info("using stub LLM (set LLM_MODE=chat to use Chat Completion API)")
		return llm.NewStubClient()
	}
}
