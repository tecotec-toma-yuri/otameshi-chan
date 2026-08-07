package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/otameshi/backend/internal/config"
)

type VoicevoxTTS struct {
	baseURL    string
	httpClient *http.Client
}

func NewVoicevoxTTS(baseURL string) *VoicevoxTTS {
	return &VoicevoxTTS{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (v *VoicevoxTTS) SynthesizeWithLang(ctx context.Context, text string, _ string) ([]byte, error) {
	return v.Synthesize(ctx, text)
}

func (v *VoicevoxTTS) Synthesize(ctx context.Context, text string) ([]byte, error) {
	started := time.Now()
	cfg := config.Get()
	speakerID := cfg.VoicevoxSpeakerID

	queryURL := fmt.Sprintf("%s/audio_query?text=%s&speaker=%d",
		v.baseURL, url.QueryEscape(text), speakerID)
	queryReq, err := http.NewRequestWithContext(ctx, http.MethodPost, queryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create audio_query request: %w", err)
	}

	queryResp, err := v.httpClient.Do(queryReq)
	if err != nil {
		slog.Warn("VOICEVOX audio_query failed, falling back to stub", "error", err)
		return NewStubTTS().Synthesize(ctx, text)
	}
	defer queryResp.Body.Close()

	if queryResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(queryResp.Body)
		slog.Warn("VOICEVOX audio_query non-200", "status", queryResp.StatusCode, "body", string(body))
		return NewStubTTS().Synthesize(ctx, text)
	}

	var queryData json.RawMessage
	if err := json.NewDecoder(queryResp.Body).Decode(&queryData); err != nil {
		return nil, fmt.Errorf("decode audio_query: %w", err)
	}

	var queryMap map[string]interface{}
	if err := json.Unmarshal(queryData, &queryMap); err == nil {
		if cfg.VoicevoxSpeedScale > 0 {
			queryMap["speedScale"] = cfg.VoicevoxSpeedScale
		}
		queryData, _ = json.Marshal(queryMap)
	}

	synthURL := fmt.Sprintf("%s/synthesis?speaker=%d", v.baseURL, speakerID)
	synthReq, err := http.NewRequestWithContext(ctx, http.MethodPost, synthURL, bytes.NewReader(queryData))
	if err != nil {
		return nil, fmt.Errorf("create synthesis request: %w", err)
	}
	synthReq.Header.Set("Content-Type", "application/json")

	synthResp, err := v.httpClient.Do(synthReq)
	if err != nil {
		slog.Warn("VOICEVOX synthesis failed, falling back to stub", "error", err)
		return NewStubTTS().Synthesize(ctx, text)
	}
	defer synthResp.Body.Close()

	if synthResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(synthResp.Body)
		slog.Warn("VOICEVOX synthesis non-200", "status", synthResp.StatusCode, "body", string(body))
		return NewStubTTS().Synthesize(ctx, text)
	}

	data, err := io.ReadAll(synthResp.Body)
	if err != nil {
		return nil, fmt.Errorf("read synthesis response: %w", err)
	}

	opus, err := WavToOpus(ctx, data)
	if err != nil {
		slog.Warn("Opus encode failed, returning WAV", "error", err)
		return data, nil
	}

	slog.Info("VOICEVOX synthesis",
		"duration_ms", time.Since(started).Milliseconds(),
		"speaker_id", speakerID,
		"text_len", len([]rune(text)),
		"wav_bytes", len(data),
		"opus_bytes", len(opus),
	)
	return opus, nil
}
