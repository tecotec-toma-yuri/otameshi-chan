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

type GroqSTT struct {
	apiKey     string
	httpClient *http.Client
}

func NewGroqSTT() *GroqSTT {
	cfg := config.Get()
	apiKey := cfg.GroqAPIKey
	if apiKey == "" {
		apiKey = cfg.OpenAIAPIKey
	}
	return &GroqSTT{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
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

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.groq.com/openai/v1/audio/transcriptions", &body)
	if err != nil {
		return TranscribeResult{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+g.apiKey)

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return TranscribeResult{}, fmt.Errorf("groq stt request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return TranscribeResult{}, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return TranscribeResult{}, fmt.Errorf("groq stt returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Text     string `json:"text"`
		Language string `json:"language"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return TranscribeResult{}, fmt.Errorf("decode response: %w", err)
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
