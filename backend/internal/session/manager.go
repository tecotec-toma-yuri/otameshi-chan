package session

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"os"
	"sync"
	"time"

	"github.com/otameshi/backend/internal/catalog"
	"github.com/otameshi/backend/internal/config"
	"github.com/otameshi/backend/internal/guardrail"
	"github.com/otameshi/backend/internal/history"
	"github.com/otameshi/backend/internal/openai"
	"github.com/otameshi/backend/internal/protocol"
	"github.com/otameshi/backend/internal/stt"
	"github.com/otameshi/backend/internal/textbuf"
	"github.com/otameshi/backend/internal/tts"
)

func generateUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

type Manager struct {
	mu            sync.Mutex
	sessionID     string
	config        protocol.SessionConfig
	state         *StateMachine
	silenceTimer  *SilenceTimer
	aiClient      openai.RealtimeClient
	ttsService    tts.TTSService
	sttService    stt.STTService
	guard         guardrail.Monitor
	sendCh        chan protocol.OutboundMessage
	cancelCurrent context.CancelFunc
	generation    uint64

	seqIndex          int
	detectedLang      string
	startedAt         time.Time
	restoredHistory   []openai.ChatMessage
	closed            bool
	interestTriggered bool
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

func newLLMClient() openai.RealtimeClient {
	mode := os.Getenv("LLM_MODE")
	switch mode {
	case "chat":
		slog.Info("using Chat Completion API")
		return openai.NewChatCompletionClient()
	default:
		slog.Info("using stub LLM (set LLM_MODE=chat to use Chat Completion API)")
		return openai.NewStubClient()
	}
}

func NewManager(config protocol.SessionConfig) *Manager {
	m := &Manager{
		sessionID:  generateUUID(),
		config:     config,
		state:      NewStateMachine(),
		aiClient:   newLLMClient(),
		ttsService: newTTSService(),
		sttService: newSTTService(),
		guard:      guardrail.NewLLMMonitor(),
		sendCh:     make(chan protocol.OutboundMessage, 64),
		startedAt:  time.Now(),
	}

	m.silenceTimer = NewSilenceTimer(
		m.handleSilenceConfirmation,
		m.handleSilenceClose,
	)

	return m
}

func (m *Manager) SessionID() string {
	return m.sessionID
}

func (m *Manager) RestoreHistory(messages []openai.ChatMessage) {
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
	m.startLLMPipeline(requestID, pipelineStart, 0, "", func(ctx context.Context) (<-chan openai.StreamEvent, error) {
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
		histMsgs := make([]history.Message, 0, len(msgs))
		for _, msg := range msgs {
			if msg.Role == "system" {
				continue
			}
			histMsgs = append(histMsgs, history.Message{Role: msg.Role, Content: msg.Content})
		}
		if err := history.Save(m.sessionID, m.startedAt, histMsgs); err != nil {
			slog.Error("failed to save session history", "error", err)
		} else {
			slog.Info("session history saved", "session_id", m.sessionID)
		}
	}

	m.aiClient.Close()
	m.state.Transition(StateClosed)

	slog.Info("session closed", "session_id", m.sessionID, "reason", "websocket_disconnected")
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

// interruptIfBusy cancels the in-flight LLM/TTS pipeline when the user
// sends a new message during Processing or AISpeaking.
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

func (m *Manager) handleSessionControl(ctrl protocol.SessionControl) {
	if ctrl.Action == "close" {
		reason := ctrl.Reason
		if reason == "" {
			reason = "client_requested"
		}
		m.initiateClose(reason, "")
	}
}

func (m *Manager) processUserMessage(text string, requestID string, pipelineStart time.Time, sttMs int64) {
	m.startLLMPipeline(requestID, pipelineStart, sttMs, text, func(ctx context.Context) (<-chan openai.StreamEvent, error) {
		return m.aiClient.SendUserMessage(ctx, text)
	})
}

func (m *Manager) startLLMPipeline(
	requestID string,
	pipelineStart time.Time,
	sttMs int64,
	userText string,
	start func(ctx context.Context) (<-chan openai.StreamEvent, error),
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

		buf := textbuf.New(func(sentence string) {
			if !m.isCurrentGeneration(gen) {
				return
			}
			// Filter out function call markup that some models emit as text content
			clean := strings.TrimSpace(sentence)
			if strings.Contains(clean, "**Function call:**") ||
				strings.Contains(clean, "**Function Call:**") ||
				strings.Contains(clean, "\"function\":") ||
				strings.HasPrefix(clean, "{\"name\":") ||
				strings.HasPrefix(clean, "**Response:**") {
				return
			}
			// Strip "**Response:** " prefix if it's part of a normal sentence
			sentence = strings.TrimPrefix(sentence, "**Response:** ")
			sentence = strings.TrimPrefix(sentence, "**Response:**\n")
			m.send(protocol.TypeTextDelta, protocol.TextDelta{Text: sentence})
			// Enqueue for TTS synthesis
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

				// Wait for all TTS sentences to finish
				<-ttsDone

				select {
				case <-ctx.Done():
					return
				default:
				}
				if !m.isCurrentGeneration(gen) {
					return
				}

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

				m.state.Transition(StateListening)
				m.silenceTimer.Resume()
				resumed = true

			case "function_call":
				if !m.isCurrentGeneration(gen) {
					buf.Reset()
					close(ttsQueue)
					return
				}

				llmMs := time.Since(llmStart).Milliseconds()

				// When LLM returns text + tool_calls together (e.g. assess_interest),
				// event.Text contains the cleaned conversation text. Flush it as a
				// proper text_done so the frontend bubble finalizes correctly.
				if event.Text != "" {
					buf.Flush()
					close(ttsQueue)
					<-ttsDone

					m.send(protocol.TypeTextDone, protocol.TextDone{
						Text:    event.Text,
						IsFinal: true,
					})
				} else {
					buf.Reset()
					close(ttsQueue)
					<-ttsDone
				}

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

func (m *Manager) isCurrentGeneration(gen uint64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.generation == gen
}

func (m *Manager) handleFunctionCall(ctx context.Context, event openai.StreamEvent) {
	switch event.FunctionName {
	case "assess_interest":
		var args openai.AssessInterestArgs
		if err := json.Unmarshal(event.FunctionArgs, &args); err != nil {
			slog.Error("failed to parse assess_interest args", "error", err)
			return
		}
		m.handleAssessInterest(ctx, args)

	case "recommend_product":
		var args openai.RecommendProductArgs
		if err := json.Unmarshal(event.FunctionArgs, &args); err != nil {
			slog.Error("failed to parse recommend_product args", "error", err)
			return
		}
		m.handleRecommendProduct(ctx, args)

	case "end_conversation":
		var args openai.EndConversationArgs
		if err := json.Unmarshal(event.FunctionArgs, &args); err != nil {
			slog.Error("failed to parse end_conversation args", "error", err)
			return
		}
		m.initiateClose("conversation_ended", args.ClosingSpeech)
	}
}

func (m *Manager) handleAssessInterest(ctx context.Context, args openai.AssessInterestArgs) {
	cfg := config.Get()
	threshold := cfg.InterestThreshold
	if threshold <= 0 {
		threshold = 3
	}

	slog.Info("interest_assessed",
		"session_id", m.sessionID,
		"interest_level", args.InterestLevel,
		"threshold", threshold,
		"detected_preferences", args.DetectedPreferences,
		"trigger_utterance", args.TriggerUtterance,
	)

	if args.InterestLevel < threshold {
		m.state.Transition(StateListening)
		m.silenceTimer.Resume()
		return
	}

	m.mu.Lock()
	m.interestTriggered = true
	m.mu.Unlock()

	if args.ResponseText != "" {
		audioData, err := m.ttsService.SynthesizeWithLang(ctx, args.ResponseText, m.getDetectedLang())
		if err != nil {
			slog.Warn("TTS for interest response failed", "error", err)
		}
		audioBase64 := ""
		if len(audioData) > 0 {
			audioBase64 = base64.StdEncoding.EncodeToString(audioData)
		}
		m.send(protocol.TypeTextDone, protocol.TextDone{
			Text:       args.ResponseText,
			AudioChunk: audioBase64,
			IsFinal:    false,
		})
	}

	catalogText := openai.BuildProductCatalogPrompt(cfg.Products)
	m.aiClient.InjectProductCatalog(catalogText)
	m.aiClient.SetRecommendationTools(cfg.Products)

	slog.Info("stage2_triggered",
		"session_id", m.sessionID,
		"interest_level", args.InterestLevel,
		"detected_preferences", args.DetectedPreferences,
	)

	requestID := generateUUID()[:8]
	pipelineStart := time.Now()
	m.startLLMPipeline(requestID, pipelineStart, 0, "", func(ctx context.Context) (<-chan openai.StreamEvent, error) {
		return m.aiClient.RequestRecommendation(ctx)
	})
}

func (m *Manager) handleRecommendProduct(ctx context.Context, args openai.RecommendProductArgs) {
	product := catalog.GetProduct(args.ProductID)
	if product == nil {
		slog.Error("product not found", "product_id", args.ProductID)
		m.state.Transition(StateListening)
		m.silenceTimer.Resume()
		return
	}

	productInfo := protocol.ProductInfo{
		ProductID:   product.ID,
		Name:        product.Name,
		Description: product.Description,
		ImageURL:    product.ImageURL,
		Tags:        product.Tags,
	}

	audioData, err := m.ttsService.SynthesizeWithLang(ctx, args.IntroductionSpeech, m.getDetectedLang())
	if err != nil {
		slog.Error("TTS synthesis failed", "error", err)
	}
	audioBase64 := ""
	if len(audioData) > 0 {
		audioBase64 = base64.StdEncoding.EncodeToString(audioData)
	}

	m.send(protocol.TypeProductRecommendation, protocol.ProductRecommendation{
		Transcript: args.IntroductionSpeech,
		AudioChunk: audioBase64,
		Reason:     args.Reason,
		Products:   []protocol.ProductInfo{productInfo},
	})

	slog.Info("product_recommended",
		"session_id", m.sessionID,
		"product_id", args.ProductID,
		"reason", args.Reason,
		"introduction_speech", args.IntroductionSpeech,
	)

	m.state.Transition(StateListening)
	m.silenceTimer.Resume()

	m.handlePostRecommendation(ctx, args.ProductID)
}

func (m *Manager) handlePostRecommendation(ctx context.Context, recommendedProductID string) {
	if m.config.RecommendationMode == "sequential" {
		product := catalog.GetProduct(recommendedProductID)
		if product != nil && len(product.RelatedProductIDs) > 0 {
			for _, relatedID := range product.RelatedProductIDs {
				select {
				case <-ctx.Done():
					return
				default:
				}
				m.recommendSequentialProduct(ctx, relatedID)
			}
		}
	}

	if m.config.PostRecommendationBehavior == "ask_interest" {
		followUp := "ご紹介した商品はいかがですか？気になるものはありましたか？"
		audioData, _ := m.ttsService.SynthesizeWithLang(ctx, followUp, m.getDetectedLang())
		audioBase64 := ""
		if len(audioData) > 0 {
			audioBase64 = base64.StdEncoding.EncodeToString(audioData)
		}
		m.send(protocol.TypeTextDone, protocol.TextDone{
			Text:       followUp,
			AudioChunk: audioBase64,
			IsFinal:    true,
		})
		m.aiClient.AppendAssistantMessage(followUp)
	}
}

func (m *Manager) recommendSequentialProduct(ctx context.Context, productID string) {
	requestID := generateUUID()[:8]
	pipelineStart := time.Now()
	m.startLLMPipeline(requestID, pipelineStart, 0, "", func(ctx context.Context) (<-chan openai.StreamEvent, error) {
		return m.aiClient.RequestSequentialRecommendation(ctx, productID)
	})
}

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

func (m *Manager) initiateClose(reason, llmClosingSpeech string) {
	if err := m.state.Transition(StateClosing); err != nil {
		return
	}
	m.silenceTimer.Stop()

	slog.Info("session closing", "session_id", m.sessionID, "reason", reason)

	speech := llmClosingSpeech
	if speech == "" {
		speech = closingSpeech(reason)
	}

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
		Message: speech,
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
