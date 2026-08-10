package factory

import (
	"github.com/otameshi/backend/internal/external/guardrail"
	"github.com/otameshi/backend/internal/external/llm"
	"github.com/otameshi/backend/internal/external/stt"
	"github.com/otameshi/backend/internal/external/tts"
)

// ChatDependencies bundles the constructors for swappable external dependencies
// needed to run a chat session, so callers can pass them around as a
// single unit instead of four separate functions.
type ChatDependencies struct {
	NewLLMClient        func() llm.RealtimeClient
	NewTTSService       func() tts.TTSService
	NewSTTService       func() stt.STTService
	NewGuardrailMonitor func() guardrail.GuardrailMonitor
}

// NewChatDependencies returns a ChatDependencies bundle wired to this package's factory functions.
func NewChatDependencies() *ChatDependencies {
	return &ChatDependencies{
		NewLLMClient:        NewLLMClient,
		NewTTSService:       NewTTSService,
		NewSTTService:       NewSTTService,
		NewGuardrailMonitor: NewGuardrailMonitor,
	}
}
