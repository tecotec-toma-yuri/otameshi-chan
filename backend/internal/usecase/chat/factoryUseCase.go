package chat

import (
	"log/slog"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/otameshi/backend/internal/config"
	"github.com/otameshi/backend/internal/external/llm"
	"github.com/otameshi/backend/internal/external/stt"
	"github.com/otameshi/backend/internal/external/tts"
	"github.com/otameshi/backend/internal/protocol"
	"github.com/otameshi/backend/internal/service"
)

func generateUUID() string {
	return uuid.New().String()
}

func newTTSService() tts.TTSService {
	cfg := config.Get()
	engine := cfg.TTSEngine
	if engine == "" {
		engine = os.Getenv("TTS_ENGINE")
	}
	if engine == "" {
		engine = "piper"
	}

	switch engine {
	case "voicevox":
		url := os.Getenv("VOICEVOX_URL")
		if url == "" {
			url = "http://voicevox:50021"
		}
		slog.Info("using VOICEVOX TTS", "url", url)
		return tts.NewVoicevoxTTS(url)
	case "piper":
		if url := os.Getenv("TTS_URL"); url != "" {
			slog.Info("using Piper TTS", "url", url)
			return tts.NewPiperTTS(url)
		}
		slog.Info("using stub TTS (set TTS_URL to use Piper)")
		return tts.NewStubTTS()
	default:
		slog.Info("using stub TTS", "engine", engine)
		return tts.NewStubTTS()
	}
}

func newSTTService() stt.STTService {
	cfg := config.Get()
	engine := cfg.STTEngine
	if engine == "" {
		engine = os.Getenv("STT_ENGINE")
	}
	if engine == "" {
		engine = "whisper"
	}

	switch engine {
	case "groq":
		slog.Info("using Groq STT")
		return stt.NewGroqSTT()
	case "whisper":
		if url := os.Getenv("STT_URL"); url != "" {
			slog.Info("using Whisper STT", "url", url)
			return stt.NewWhisperSTT(url)
		}
		slog.Info("using stub STT (set STT_URL to use Whisper)")
		return stt.NewStubSTT()
	default:
		slog.Info("unknown STT engine, using stub", "engine", engine)
		return stt.NewStubSTT()
	}
}

func newLLMClient() llm.RealtimeClient {
	mode := os.Getenv("LLM_MODE")
	switch mode {
	case "chat":
		slog.Info("using Chat Completion API")
		return llm.NewChatCompletionClient()
	default:
		slog.Info("using stub LLM (set LLM_MODE=chat to use Chat Completion API)")
		return llm.NewStubClient()
	}
}

func NewManager(cfg protocol.SessionConfig) *Manager {
	m := &Manager{
		sessionID:  generateUUID(),
		config:     cfg,
		state:      NewStateMachine(),
		aiClient:   newLLMClient(),
		ttsService: newTTSService(),
		sttService: newSTTService(),
		guard:      service.NewGuardrailMonitor(),
		sendCh:     make(chan protocol.OutboundMessage, 64),
		startedAt:  time.Now(),
	}

	m.silenceTimer = NewSilenceTimer(
		m.handleSilenceConfirmation,
		m.handleSilenceClose,
	)

	return m
}
