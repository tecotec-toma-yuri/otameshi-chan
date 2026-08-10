package factory

import (
	"log/slog"

	"github.com/otameshi/backend/internal/config"
	"github.com/otameshi/backend/internal/external/tts"
)

func NewTTSService() tts.TTSService {
	cfg := config.Get()
	infra := config.Infra()
	engine := cfg.TTSEngine
	if engine == "" {
		engine = "piper"
	}

	switch engine {
	case "voicevox":
		slog.Info("using VOICEVOX TTS", "url", infra.VoicevoxURL)
		return tts.NewVoicevoxTTS(infra.VoicevoxURL)
	case "piper":
		if infra.TTSURL != "" {
			slog.Info("using Piper TTS", "url", infra.TTSURL)
			return tts.NewPiperTTS(infra.TTSURL)
		}
		slog.Info("using stub TTS (set TTS_URL to use Piper)")
		return tts.NewStubTTS()
	default:
		slog.Info("using stub TTS", "engine", engine)
		return tts.NewStubTTS()
	}
}
