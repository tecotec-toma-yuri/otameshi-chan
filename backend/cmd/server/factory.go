package main

import (
	"log/slog"

	"github.com/otameshi/backend/internal/config"
	"github.com/otameshi/backend/internal/external/llm"
	"github.com/otameshi/backend/internal/external/stt"
	"github.com/otameshi/backend/internal/external/tts"
	"github.com/otameshi/backend/internal/service"
)

func newLLMClient() llm.RealtimeClient {
	switch config.Get().LLMMode {
	case "chat":
		slog.Info("using Chat Completion API")
		return llm.NewChatCompletionClient()
	default:
		slog.Info("using stub LLM (set LLM_MODE=chat to use Chat Completion API)")
		return llm.NewStubClient()
	}
}

func newTTSService() tts.TTSService {
	cfg := config.Get()
	engine := cfg.TTSEngine
	if engine == "" {
		engine = "piper"
	}

	switch engine {
	case "voicevox":
		slog.Info("using VOICEVOX TTS", "url", cfg.VoicevoxURL)
		return tts.NewVoicevoxTTS(cfg.VoicevoxURL)
	case "piper":
		if cfg.TTSURL != "" {
			slog.Info("using Piper TTS", "url", cfg.TTSURL)
			return tts.NewPiperTTS(cfg.TTSURL)
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
		engine = "whisper"
	}

	switch engine {
	case "groq":
		slog.Info("using Groq STT")
		return stt.NewGroqSTT()
	case "whisper":
		if cfg.STTURL != "" {
			slog.Info("using Whisper STT", "url", cfg.STTURL)
			return stt.NewWhisperSTT(cfg.STTURL)
		}
		slog.Info("using stub STT (set STT_URL to use Whisper)")
		return stt.NewStubSTT()
	default:
		slog.Info("unknown STT engine, using stub", "engine", engine)
		return stt.NewStubSTT()
	}
}

func newGuardrailMonitor() service.GuardrailMonitor {
	return service.NewGuardrailMonitor()
}
