package stt

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"mime/multipart"
	"strings"
	"time"

	"github.com/otameshi/backend/internal/config"
	"github.com/otameshi/backend/internal/external/httpclient"
)

type GroqSTT struct {
	httpclient.BaseClient
	apiKey string
}

func NewGroqSTT() *GroqSTT {
	infra := config.Infra()
	apiKey := infra.GroqAPIKey
	if apiKey == "" {
		apiKey = infra.OpenAIAPIKey
	}
	return &GroqSTT{
		BaseClient: httpclient.NewBaseClient("https://api.groq.com/openai/v1", 60*time.Second),
		apiKey:     apiKey,
	}
}

func (g *GroqSTT) Transcribe(ctx context.Context, audioData []byte) (TranscribeResult, error) {
	started := time.Now()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("file", "audio.webm")
	if err != nil {
		return TranscribeResult{}, fmt.Errorf("create form file: %w", err)
	}
	if _, err := part.Write(audioData); err != nil {
		return TranscribeResult{}, fmt.Errorf("write audio data: %w", err)
	}

	cfg := config.Get()
	sttModel := cfg.STTModel
	if sttModel == "" {
		sttModel = "whisper-large-v3-turbo"
	}
	_ = writer.WriteField("model", sttModel)
	_ = writer.WriteField("response_format", "verbose_json")
	_ = writer.WriteField("temperature", "0.0")
	lang := cfg.STTLanguage
	if lang != "" && lang != "auto" {
		_ = writer.WriteField("language", lang)
	}

	if cfg.STTPrompt != "" {
		_ = writer.WriteField("prompt", cfg.STTPrompt)
	}

	writer.Close()

	resp, err := g.Post(ctx, g.BaseURL+"/audio/transcriptions", writer.FormDataContentType(), &body, g.apiKey)
	if err != nil {
		return TranscribeResult{}, fmt.Errorf("groq stt request failed: %w", err)
	}

	var result struct {
		Text     string `json:"text"`
		Language string `json:"language"`
	}
	if err := httpclient.DecodeJSON(resp, &result); err != nil {
		return TranscribeResult{}, fmt.Errorf("groq stt: %w", err)
	}

	text := strings.TrimSpace(result.Text)
	detectedLang := result.Language
	if detectedLang == "" && lang != "auto" {
		detectedLang = lang
	}

	slog.Info("STT transcription (Groq)",
		"text", text,
		"language", detectedLang,
		"duration_ms", time.Since(started).Milliseconds(),
		"wav_bytes", len(audioData),
	)
	return TranscribeResult{Text: text, Language: detectedLang}, nil
}
