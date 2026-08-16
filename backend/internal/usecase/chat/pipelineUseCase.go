package chat

import (
	"context"
	"encoding/base64"
	"log/slog"
	"sync"
	"time"

	"github.com/otameshi/backend/internal/external/llm"
	"github.com/otameshi/backend/internal/protocol"
)

func (m *Manager) processUserMessage(text string, requestID string, pipelineStart time.Time, sttMs int64) {
	m.startLLMPipeline(requestID, pipelineStart, sttMs, text, func(ctx context.Context) (<-chan llm.StreamEvent, error) {
		return m.aiClient.SendUserMessage(ctx, text)
	})
}

func (m *Manager) startLLMPipeline(
	requestID string,
	pipelineStart time.Time,
	sttMs int64,
	userText string,
	start func(ctx context.Context) (<-chan llm.StreamEvent, error),
) {
	ctx, cancel := context.WithCancel(context.Background())

	m.mu.Lock()
	if m.cancelCurrent != nil {
		m.cancelCurrent()
	}
	m.generation++
	gen := m.generation
	m.cancelCurrent = cancel
	m.mu.Unlock()

	llmStart := time.Now()
	eventCh, err := start(ctx)
	if err != nil {
		m.sendError("ai_error", "Failed to process message", true)
		if m.isCurrentGeneration(gen) {
			m.state.Transition(StateListening)
			m.silenceTimer.Resume()
		}
		cancel()
		return
	}

	go func() {
		defer cancel()
		resumed := false
		defer func() {
			if !resumed && m.isCurrentGeneration(gen) {
				m.state.Transition(StateListening)
				m.silenceTimer.Resume()
			}
		}()

		var fullText string
		var firstDeltaAt time.Time
		var sentenceID int
		var ttsMu sync.Mutex
		var totalTTSMs int64
		ttsDone := make(chan struct{})
		ttsQueue := make(chan string, 32)

		// TTS worker: synthesizes sentences in order and streams audio
		go func() {
			defer close(ttsDone)
			for sentence := range ttsQueue {
				if !m.isCurrentGeneration(gen) {
					return
				}
				select {
				case <-ctx.Done():
					return
				default:
				}

				ttsStart := time.Now()
				audioData, err := m.ttsService.SynthesizeWithLang(ctx, sentence, m.getDetectedLang())
				elapsed := time.Since(ttsStart).Milliseconds()

				ttsMu.Lock()
				sentenceID++
				sid := sentenceID
				totalTTSMs += elapsed
				ttsMu.Unlock()

				if err != nil {
					slog.Error("TTS synthesis failed", "error", err, "request_id", requestID, "sentence", sentence)
					continue
				}

				if !m.isCurrentGeneration(gen) {
					return
				}

				if len(audioData) > 0 {
					audioBase64 := base64.StdEncoding.EncodeToString(audioData)
					m.send(protocol.TypeAudioStream, protocol.AudioStream{
						Audio:      audioBase64,
						SentenceID: sid,
					})
					slog.Info("tts_sentence",
						"request_id", requestID,
						"sentence_id", sid,
						"duration_ms", elapsed,
						"text_len", len([]rune(sentence)),
						"audio_bytes", len(audioData),
					)
				}
			}
		}()

		buf := New(func(sentence string) {
			if !m.isCurrentGeneration(gen) {
				return
			}
			m.send(protocol.TypeTextDelta, protocol.TextDelta{Text: sentence})
			select {
			case ttsQueue <- sentence:
			case <-ctx.Done():
			}
		})

		if !m.isCurrentGeneration(gen) {
			buf.Reset()
			close(ttsQueue)
			return
		}

		m.state.Transition(StateAISpeaking)
		m.silenceTimer.Pause()

		for event := range eventCh {
			if !m.isCurrentGeneration(gen) {
				buf.Reset()
				close(ttsQueue)
				return
			}

			select {
			case <-ctx.Done():
				buf.Reset()
				close(ttsQueue)
				return
			default:
			}

			switch event.Type {
			case "text_delta":
				if firstDeltaAt.IsZero() {
					firstDeltaAt = time.Now()
					slog.Info("latency",
						"request_id", requestID,
						"stage", "llm_first_delta",
						"duration_ms", time.Since(llmStart).Milliseconds(),
						"since_speech_ms", time.Since(pipelineStart).Milliseconds(),
					)
				}
				fullText += event.Text
				buf.Add(event.Text)

			case "text_done":
				if !m.isCurrentGeneration(gen) {
					buf.Reset()
					close(ttsQueue)
					return
				}

				llmMs := time.Since(llmStart).Milliseconds()
				buf.Flush()
				close(ttsQueue)
				go m.checkGuardrail(event.Text)

				<-ttsDone

				// TTS が中断されても、生成済みのテキストは確定させる。
				// text_done を送らないとクライアント側のストリーミング表示が終わらない。
				ttsMu.Lock()
				ttsMs := totalTTSMs
				ttsMu.Unlock()

				totalMs := time.Since(pipelineStart).Milliseconds()
				ttfbMs := int64(0)
				if !firstDeltaAt.IsZero() {
					ttfbMs = firstDeltaAt.Sub(llmStart).Milliseconds()
				}
				latency := &protocol.LatencyInfo{
					STTMs:      sttMs,
					LLMTTFBMs:  ttfbMs,
					LLMTotalMs: llmMs,
					TTSMs:      ttsMs,
					TotalMs:    totalMs,
				}

				m.send(protocol.TypeTextDone, protocol.TextDone{
					Text:    fullText,
					IsFinal: true,
					Latency: latency,
				})

				slog.Info("latency_summary",
					"request_id", requestID,
					"stt_ms", latency.STTMs,
					"llm_ttfb_ms", latency.LLMTTFBMs,
					"llm_total_ms", latency.LLMTotalMs,
					"tts_ms", latency.TTSMs,
					"total_ms", latency.TotalMs,
					"user_text", userText,
					"assistant_text", fullText,
				)

				// 割り込み後は後続の世代が状態を管理しているため上書きしない
				if m.isCurrentGeneration(gen) {
					m.state.Transition(StateListening)
					m.silenceTimer.Resume()
				}
				resumed = true

			case "function_call":
				if !m.isCurrentGeneration(gen) {
					buf.Reset()
					close(ttsQueue)
					return
				}

				llmMs := time.Since(llmStart).Milliseconds()
				buf.Reset()
				close(ttsQueue)
				<-ttsDone
				ttsStart := time.Now()
				m.handleFunctionCall(ctx, event)
				resumed = true
				ttsMs := time.Since(ttsStart).Milliseconds()
				slog.Info("latency_summary",
					"request_id", requestID,
					"stt_ms", sttMs,
					"llm_total_ms", llmMs,
					"tts_ms", ttsMs,
					"total_ms", time.Since(pipelineStart).Milliseconds(),
					"function", event.FunctionName,
					"user_text", userText,
				)

			case "error":
				buf.Reset()
				close(ttsQueue)
				<-ttsDone
				m.sendError("ai_error", "LLMからエラーが返されました。しばらくしてから再度お試しください。", true)
				return
			}
		}
	}()
}

func (m *Manager) handleAudioChunk(chunk protocol.AudioChunk) {
	m.interruptIfBusy()

	m.silenceTimer.Reset()

	text := chunk.Text
	if text == "" {
		return
	}

	go m.checkGuardrail(text)

	m.state.Transition(StateProcessing)
	requestID := generateUUID()[:8]
	m.processUserMessage(text, requestID, time.Now(), 0)
}

func (m *Manager) handleAudioSpeech(audioBase64 string) {
	if audioBase64 == "" {
		return
	}

	m.interruptIfBusy()

	m.silenceTimer.Reset()
	m.state.Transition(StateProcessing)

	go func() {
		pipelineStart := time.Now()
		requestID := generateUUID()[:8]

		slog.Info("latency",
			"request_id", requestID,
			"stage", "speech_received",
			"audio_b64_len", len(audioBase64),
			"duration_ms", 0,
		)

		decodeStart := time.Now()
		audioData, err := base64.StdEncoding.DecodeString(audioBase64)
		if err != nil {
			slog.Error("failed to decode audio", "error", err, "request_id", requestID)
			m.sendError("stt_error", "音声データの処理に失敗しました", true)
			m.state.Transition(StateListening)
			return
		}

		slog.Info("latency",
			"request_id", requestID,
			"stage", "decode_b64",
			"duration_ms", time.Since(decodeStart).Milliseconds(),
			"audio_bytes", len(audioData),
		)

		sttStart := time.Now()
		sttResult, err := m.sttService.Transcribe(context.Background(), audioData)
		sttMs := time.Since(sttStart).Milliseconds()
		if err != nil {
			slog.Error("STT transcription failed", "error", err, "request_id", requestID, "stt_ms", sttMs)
			m.send(protocol.TypeUserTranscript, protocol.UserTranscript{Skipped: true})
			m.sendError("stt_error", "音声認識に失敗しました", true)
			m.state.Transition(StateListening)
			return
		}

		text := sttResult.Text
		if sttResult.Language != "" {
			m.mu.Lock()
			m.detectedLang = sttResult.Language
			m.mu.Unlock()
		}

		slog.Info("latency",
			"request_id", requestID,
			"stage", "stt",
			"duration_ms", sttMs,
			"text", text,
			"language", sttResult.Language,
		)

		if text == "" || isNoiseText(text) {
			slog.Info("latency",
				"request_id", requestID,
				"stage", "stt_noise_skip",
				"duration_ms", time.Since(pipelineStart).Milliseconds(),
				"text", text,
			)
			m.send(protocol.TypeUserTranscript, protocol.UserTranscript{Skipped: true})
			m.state.Transition(StateListening)
			return
		}

		m.send(protocol.TypeUserTranscript, protocol.UserTranscript{Text: text})

		go m.checkGuardrail(text)
		m.processUserMessage(text, requestID, pipelineStart, sttMs)
	}()
}

func (m *Manager) interruptIfBusy() {
	currentState := m.state.Current()
	if currentState != StateAISpeaking && currentState != StateProcessing {
		return
	}

	slog.Info("barge-in detected", "session_id", m.sessionID, "state", currentState.String())
	m.mu.Lock()
	if m.cancelCurrent != nil {
		m.cancelCurrent()
		m.cancelCurrent = nil
	}
	m.mu.Unlock()

	m.send(protocol.TypeClearAudioBuffer, protocol.ClearAudioBuffer{
		Reason: "barge_in",
	})
	m.state.Transition(StateListening)
}

func isNoiseText(text string) bool {
	noisePatterns := []string{
		"(音楽)", "(拍手)", "(笑)", "...", "ご視聴ありがとうございました",
		"字幕", "翻訳", "(無音)", "MBS", "ありがとうございました。",
		"ご視聴ありがとうございました。", "お疲れ様でした。",
		"おやすみなさい", "おやすみなさい。",
		"Thanks for watching!", "Thank you for watching!",
		"Bye!", "Bye-bye!", "See you!",
	}
	for _, p := range noisePatterns {
		if text == p {
			return true
		}
	}
	if len(text) <= 3 {
		return true
	}
	return false
}

func (m *Manager) getDetectedLang() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.detectedLang != "" {
		return m.detectedLang
	}
	return ""
}
