package stt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/otameshi/backend/internal/config"
)

type WhisperSTT struct {
	baseURL    string
	language   string
	httpClient *http.Client
}

func NewWhisperSTT(baseURL string) *WhisperSTT {
	return &WhisperSTT{
		baseURL:  baseURL,
		language: "ja",
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (w *WhisperSTT) Transcribe(ctx context.Context, audioData []byte) (TranscribeResult, error) {
	started := time.Now()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("file", "audio.wav")
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

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.baseURL+"/inference", &body)
	if err != nil {
		return TranscribeResult{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return TranscribeResult{}, fmt.Errorf("whisper request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return TranscribeResult{}, fmt.Errorf("whisper returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Text     string `json:"text"`
		Language string `json:"language"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return TranscribeResult{}, fmt.Errorf("decode response: %w", err)
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
