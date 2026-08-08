package chat

import (
	"time"

	"github.com/google/uuid"
	"github.com/otameshi/backend/internal/external/llm"
	"github.com/otameshi/backend/internal/external/stt"
	"github.com/otameshi/backend/internal/external/tts"
	"github.com/otameshi/backend/internal/protocol"
	"github.com/otameshi/backend/internal/service"
)

func generateUUID() string {
	return uuid.New().String()
}

func NewManager(
	cfg protocol.SessionConfig,
	aiClient llm.RealtimeClient,
	ttsService tts.TTSService,
	sttService stt.STTService,
	guard service.GuardrailMonitor,
) *Manager {
	m := &Manager{
		sessionID:  generateUUID(),
		config:     cfg,
		state:      NewStateMachine(),
		aiClient:   aiClient,
		ttsService: ttsService,
		sttService: sttService,
		guard:      guard,
		sendCh:     make(chan protocol.OutboundMessage, 64),
		startedAt:  time.Now(),
	}
	m.silenceTimer = NewSilenceTimer(m.handleSilenceConfirmation, m.handleSilenceClose)
	return m
}
