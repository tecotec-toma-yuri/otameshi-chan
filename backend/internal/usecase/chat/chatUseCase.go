package chat

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/otameshi/backend/internal/external/guardrail"
	"github.com/otameshi/backend/internal/external/llm"
	"github.com/otameshi/backend/internal/external/stt"
	"github.com/otameshi/backend/internal/external/tts"
	"github.com/otameshi/backend/internal/protocol"
	"github.com/otameshi/backend/internal/service"
)

type Manager struct {
	mu            sync.Mutex
	sessionID     string
	config        protocol.SessionConfig
	state         *StateMachine
	silenceTimer  *SilenceTimer
	aiClient      llm.RealtimeClient
	ttsService    tts.TTSService
	sttService    stt.STTService
	guard         guardrail.GuardrailMonitor
	sendCh        chan protocol.OutboundMessage
	cancelCurrent context.CancelFunc
	generation    uint64

	seqIndex        int
	detectedLang    string
	startedAt       time.Time
	restoredHistory []llm.ChatMessage
	closed          bool
}

func (m *Manager) SessionID() string {
	return m.sessionID
}

func (m *Manager) RestoreHistory(messages []llm.ChatMessage) {
	m.aiClient.RestoreHistory(messages)
	m.restoredHistory = messages
}

func (m *Manager) SendCh() <-chan protocol.OutboundMessage {
	return m.sendCh
}

func (m *Manager) Start() {
	m.send(protocol.TypeConnectionState, protocol.ConnectionState{
		State:     "connected",
		SessionID: m.sessionID,
	})

	m.state.Transition(StateListening)
	m.silenceTimer.Start()

	if len(m.restoredHistory) > 0 {
		slog.Info("session restored", "session_id", m.sessionID, "history_messages", len(m.restoredHistory))
		go m.sendRestoredHistory()
	} else {
		slog.Info("session started", "session_id", m.sessionID, "mode", m.config.RecommendationMode)
		go m.sendGreeting()
	}
}

func (m *Manager) sendGreeting() {
	requestID := generateUUID()[:8]
	pipelineStart := time.Now()
	slog.Info("requesting greeting from LLM", "session_id", m.sessionID, "request_id", requestID)
	m.state.Transition(StateProcessing)
	m.startLLMPipeline(requestID, pipelineStart, 0, "", func(ctx context.Context) (<-chan llm.StreamEvent, error) {
		return m.aiClient.RequestGreeting(ctx)
	})
}

func (m *Manager) sendRestoredHistory() {
	msgs := make([]protocol.HistoryMessage, 0, len(m.restoredHistory))
	for _, msg := range m.restoredHistory {
		msgs = append(msgs, protocol.HistoryMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}
	m.send(protocol.TypeHistoryRestore, protocol.HistoryRestore{
		Messages: msgs,
	})

	resumeText := "前回の会話の続きですね。何かお手伝いできることはありますか？"
	audioData, _ := m.ttsService.SynthesizeWithLang(context.Background(), resumeText, m.getDetectedLang())
	audioBase64 := ""
	if len(audioData) > 0 {
		audioBase64 = base64.StdEncoding.EncodeToString(audioData)
	}
	m.send(protocol.TypeTextDone, protocol.TextDone{
		Text:       resumeText,
		AudioChunk: audioBase64,
		IsFinal:    true,
	})
	m.aiClient.AppendAssistantMessage(resumeText)
}

func (m *Manager) HandleMessage(msg protocol.InboundMessage) {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return
	}
	m.mu.Unlock()

	switch msg.Type {
	case protocol.TypeAudioChunk:
		var chunk protocol.AudioChunk
		if err := json.Unmarshal(msg.Raw, &chunk); err != nil {
			m.sendError("parse_error", "Failed to parse audio_chunk", true)
			return
		}
		m.handleAudioChunk(chunk)

	case protocol.TypeAudioSpeech:
		var speech struct {
			Audio string `json:"audio"`
		}
		if err := json.Unmarshal(msg.Raw, &speech); err != nil {
			m.sendError("parse_error", "Failed to parse audio_speech", true)
			return
		}
		m.handleAudioSpeech(speech.Audio)

	case protocol.TypeSessionControl:
		var ctrl protocol.SessionControl
		if err := json.Unmarshal(msg.Raw, &ctrl); err != nil {
			m.sendError("parse_error", "Failed to parse session_control", true)
			return
		}
		m.handleSessionControl(ctrl)

	default:
		m.sendError("unknown_type", fmt.Sprintf("Unknown message type: %s", msg.Type), true)
	}
}

func (m *Manager) handleSessionControl(ctrl protocol.SessionControl) {
	if ctrl.Action == "close" {
		reason := ctrl.Reason
		if reason == "" {
			reason = "client_requested"
		}
		m.initiateClose(reason, "")
	}
}

func (m *Manager) Close() {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return
	}
	m.closed = true
	m.mu.Unlock()

	m.silenceTimer.Stop()
	if m.cancelCurrent != nil {
		m.cancelCurrent()
	}

	defer close(m.sendCh)

	if msgs := m.aiClient.History(); len(msgs) > 0 {
		histMsgs := make([]service.HistoryMessage, 0, len(msgs))
		for _, msg := range msgs {
			if msg.Role == "system" {
				continue
			}
			histMsgs = append(histMsgs, service.HistoryMessage{Role: msg.Role, Content: msg.Content})
		}
		if err := service.SaveHistory(m.sessionID, m.startedAt, histMsgs); err != nil {
			slog.Error("failed to save session history", "error", err)
		} else {
			slog.Info("session history saved", "session_id", m.sessionID)
		}
	}

	m.aiClient.Close()
	m.state.Transition(StateClosed)

	slog.Info("session closed", "session_id", m.sessionID, "reason", "websocket_disconnected")
}

func (m *Manager) send(msgType string, payload interface{}) {
	m.mu.Lock()
	closed := m.closed
	m.mu.Unlock()
	if closed {
		return
	}
	msg := protocol.NewOutbound(msgType, payload)
	select {
	case m.sendCh <- msg:
	default:
		slog.Warn("send channel full, dropping message", "type", msgType)
	}
}

func (m *Manager) SendError(code, message string) {
	m.send(protocol.TypeError, protocol.Error{
		Code:        code,
		Message:     message,
		Recoverable: true,
	})
}

func (m *Manager) sendError(code, message string, recoverable bool) {
	m.send(protocol.TypeError, protocol.Error{
		Code:        code,
		Message:     message,
		Recoverable: recoverable,
	})
}

func (m *Manager) isCurrentGeneration(gen uint64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.generation == gen
}
