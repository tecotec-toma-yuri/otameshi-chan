package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"time"

	"github.com/otameshi/backend/internal/config"
	"github.com/otameshi/backend/internal/external/httpclient"
)

type VoicevoxTTS struct {
	httpclient.BaseClient
}

func NewVoicevoxTTS(baseURL string) *VoicevoxTTS {
	return &VoicevoxTTS{
		BaseClient: httpclient.NewBaseClient(baseURL, 60*time.Second),
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
		v.BaseURL, url.QueryEscape(text), speakerID)
	queryResp, err := v.Post(ctx, queryURL, "", nil, "")
	if err != nil {
		slog.Warn("VOICEVOX audio_query failed, falling back to stub", "error", err)
		return NewStubTTS().Synthesize(ctx, text)
	}

	var queryData json.RawMessage
	if err := httpclient.DecodeJSON(queryResp, &queryData); err != nil {
		slog.Warn("VOICEVOX audio_query returned an error, falling back to stub", "error", err)
		return NewStubTTS().Synthesize(ctx, text)
	}

	var queryMap map[string]interface{}
	if err := json.Unmarshal(queryData, &queryMap); err == nil {
		if cfg.VoicevoxSpeedScale > 0 {
			queryMap["speedScale"] = cfg.VoicevoxSpeedScale
		}
		queryData, _ = json.Marshal(queryMap)
	}

	synthURL := fmt.Sprintf("%s/synthesis?speaker=%d", v.BaseURL, speakerID)
	synthResp, err := v.Post(ctx, synthURL, "application/json", bytes.NewReader(queryData), "")
	if err != nil {
		slog.Warn("VOICEVOX synthesis failed, falling back to stub", "error", err)
		return NewStubTTS().Synthesize(ctx, text)
	}

	data, err := httpclient.ReadBody(synthResp)
	if err != nil {
		slog.Warn("VOICEVOX synthesis returned an error, falling back to stub", "error", err)
		return NewStubTTS().Synthesize(ctx, text)
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
