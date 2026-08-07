package chat

import (
	"context"
	"encoding/base64"
	"log/slog"

	"github.com/otameshi/backend/internal/protocol"
)

func (m *Manager) handleSilenceConfirmation() {
	slog.Info("silence stage 1: sending confirmation", "session_id", m.sessionID)
	ctx := context.Background()
	text := "まだいらっしゃいますか？何かお手伝いできることはありますか？"

	audioData, _ := m.ttsService.SynthesizeWithLang(ctx, text, m.getDetectedLang())
	audioBase64 := ""
	if len(audioData) > 0 {
		audioBase64 = base64.StdEncoding.EncodeToString(audioData)
	}

	m.send(protocol.TypeTextDone, protocol.TextDone{
		Text:       text,
		AudioChunk: audioBase64,
		IsFinal:    true,
	})
}

func (m *Manager) handleSilenceClose() {
	slog.Info("silence stage 2: closing session", "session_id", m.sessionID)
	m.initiateClose("silence_timeout", "")
}

func closingSpeech(reason string) string {
	switch reason {
	case "conversation_ended":
		return "ありがとうございました！またお気軽にお声がけくださいね。"
	case "silence_timeout":
		return "お時間をいただきありがとうございました。またいつでもどうぞ。"
	case "client_requested", "user_disconnect":
		return "ご利用ありがとうございました！またお会いできるのを楽しみにしています。"
	default:
		return "ありがとうございました。"
	}
}

func (m *Manager) initiateClose(reason, _ string) {
	if err := m.state.Transition(StateClosing); err != nil {
		return
	}
	m.silenceTimer.Stop()

	slog.Info("session closing", "session_id", m.sessionID, "reason", reason)

	speech := closingSpeech(reason)
	audioData, err := m.ttsService.SynthesizeWithLang(context.Background(), speech, m.getDetectedLang())
	if err != nil {
		slog.Warn("closing TTS failed", "error", err)
	}

	if len(audioData) > 0 {
		audioBase64 := base64.StdEncoding.EncodeToString(audioData)
		m.send(protocol.TypeAudioStream, protocol.AudioStream{
			Audio:      audioBase64,
			SentenceID: 0,
			IsFinal:    true,
		})
	}

	m.send(protocol.TypeTextDone, protocol.TextDone{
		Text:    speech,
		IsFinal: true,
	})

	m.send(protocol.TypeSessionClose, protocol.SessionClose{
		Reason:  reason,
		Message: "セッションが終了しました。",
	})

	m.state.Transition(StateClosed)
}

func (m *Manager) checkGuardrail(text string) {
	passed, reason, err := m.guard.Check(context.Background(), text)
	if err != nil {
		slog.Error("guardrail check failed", "error", err)
		return
	}
	if !passed {
		slog.Warn("content violation detected", "reason", reason, "session_id", m.sessionID)
		m.send(protocol.TypeContentViolation, protocol.ContentViolation{
			Reason:  reason,
			Message: "不適切なコンテンツが検出されました。",
		})
	}
}
