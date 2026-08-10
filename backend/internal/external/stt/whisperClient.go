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

type WhisperSTT struct {
	httpclient.BaseClient
	language string
}

func NewWhisperSTT(baseURL string) *WhisperSTT {
	return &WhisperSTT{
		BaseClient: httpclient.NewBaseClient(baseURL, 60*time.Second),
		language:   "ja",
	}
}

func (w *WhisperSTT) Transcribe(ctx context.Context, audioData []byte) (TranscribeResult, error) {
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
	lang := cfg.STTLanguage
	if lang == "" {
		lang = w.language
	}
	if lang != "auto" {
		_ = writer.WriteField("language", lang)
	}
	_ = writer.WriteField("response_format", "verbose_json")
	_ = writer.WriteField("temperature", "0.0")

	if cfg.STTPrompt != "" {
		_ = writer.WriteField("prompt", cfg.STTPrompt)
	}

	writer.Close()

	resp, err := w.Post(ctx, w.BaseURL+"/inference", writer.FormDataContentType(), &body, "")
	if err != nil {
		return TranscribeResult{}, fmt.Errorf("whisper request failed: %w", err)
	}

	var result struct {
		Text     string `json:"text"`
		Language string `json:"language"`
	}
	if err := httpclient.DecodeJSON(resp, &result); err != nil {
		return TranscribeResult{}, fmt.Errorf("whisper: %w", err)
	}

	text := strings.TrimSpace(result.Text)
	detectedLang := result.Language
	if detectedLang == "" {
		detectedLang = lang
	}

	slog.Info("STT transcription",
		"text", text,
		"language", detectedLang,
		"duration_ms", time.Since(started).Milliseconds(),
		"wav_bytes", len(audioData),
	)
	return TranscribeResult{Text: text, Language: detectedLang}, nil
}
