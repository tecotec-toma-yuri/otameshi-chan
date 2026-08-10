package factory

import (
	"log/slog"

	"github.com/otameshi/backend/internal/config"
	"github.com/otameshi/backend/internal/external/stt"
)

func NewSTTService() stt.STTService {
	cfg := config.Get()
	infra := config.Infra()
	engine := cfg.STTEngine
	if engine == "" {
		engine = "whisper"
	}

	switch engine {
	case "groq":
		slog.Info("using Groq STT")
		return stt.NewGroqSTT()
	case "whisper":
		if infra.STTURL != "" {
			slog.Info("using Whisper STT", "url", infra.STTURL)
			return stt.NewWhisperSTT(infra.STTURL)
		}
		slog.Info("using stub STT (set STT_URL to use Whisper)")
		return stt.NewStubSTT()
	default:
		slog.Info("unknown STT engine, using stub", "engine", engine)
		return stt.NewStubSTT()
	}
}
