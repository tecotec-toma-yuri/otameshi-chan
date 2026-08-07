package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/otameshi/backend/internal/config"
)

type PiperTTS struct {
	baseURL    string
	httpClient *http.Client
}

func NewPiperTTS(baseURL string) *PiperTTS {
	return &PiperTTS{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type synthesizeRequest struct {
	Text        string  `json:"text"`
	Voice       string  `json:"voice,omitempty"`
	SpeakerID   int     `json:"speaker_id"`
	Language    string  `json:"language,omitempty"`
	LengthScale float64 `json:"length_scale"`
	NoiseScale  float64 `json:"noise_scale"`
	NoiseW      float64 `json:"noise_w"`
}

func (p *PiperTTS) Synthesize(ctx context.Context, text string) ([]byte, error) {
	return p.SynthesizeWithLang(ctx, text, "")
}

var langFullToISO = map[string]string{
	"japanese":   "ja",
	"english":    "en",
	"chinese":    "zh",
	"spanish":    "es",
	"french":     "fr",
	"portuguese": "pt",
}

var langDefaultSpeaker = map[string]int{
	"ja": 0,
	"en": 20,
	"zh": 330,
	"es": 472,
	"fr": 535,
	"pt": 563,
}

func normalizeLang(lang string) string {
	if iso, ok := langFullToISO[lang]; ok {
		return iso
	}
	return lang
}

func (p *PiperTTS) SynthesizeWithLang(ctx context.Context, text string, language string) ([]byte, error) {
	started := time.Now()
	cfg := config.Get()
	lang := normalizeLang(language)
	if lang == "" || lang == "auto" {
		lang = cfg.TTSLanguage
	}
	voice := cfg.TTSVoice
	speakerID := cfg.TTSSpeakerID
	if lang != "" && lang != "ja" {
		if sid, ok := langDefaultSpeaker[lang]; ok {
			speakerID = sid
		}
		voice = "base_6lang"
	}
	body, err := json.Marshal(synthesizeRequest{
		Text:        text,
		Voice:       voice,
		SpeakerID:   speakerID,
		Language:    lang,
		LengthScale: cfg.TTSLengthScale,
		NoiseScale:  cfg.TTSNoiseScale,
		NoiseW:      cfg.TTSNoiseW,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/synthesize", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		slog.Warn("Piper TTS request failed, falling back to stub", "error", err)
		return NewStubTTS().Synthesize(ctx, text)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Warn("Piper TTS returned non-200", "status", resp.StatusCode)
		return NewStubTTS().Synthesize(ctx, text)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	opus, err := WavToOpus(ctx, data)
	if err != nil {
		slog.Warn("Opus encode failed, returning WAV", "error", err)
		return data, nil
	}

	slog.Info("TTS synthesis",
		"duration_ms", time.Since(started).Milliseconds(),
		"voice", cfg.TTSVoice,
		"speaker_id", cfg.TTSSpeakerID,
		"language", lang,
		"text_len", len([]rune(text)),
		"wav_bytes", len(data),
		"opus_bytes", len(opus),
	)
	return opus, nil
}
