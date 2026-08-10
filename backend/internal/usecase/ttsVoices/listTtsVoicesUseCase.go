package ttsVoices

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/otameshi/backend/internal/config"
)

type voicevoxSpeaker struct {
	Name   string          `json:"name"`
	UUID   string          `json:"speaker_uuid"`
	Styles []voicevoxStyle `json:"styles"`
}

type voicevoxStyle struct {
	Name string `json:"name"`
	ID   int    `json:"id"`
}

type SpeakerInfo struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Style string `json:"style"`
}

func GetVoices(engine string) any {
	if engine == "" {
		engine = config.Get().TTSEngine
	}

	switch engine {
	case "voicevox":
		speakers, err := fetchVoicevoxSpeakers()
		if err != nil {
			slog.Warn("failed to fetch VOICEVOX speakers", "error", err)
			return map[string]interface{}{"engine": "voicevox", "speakers": []interface{}{}}
		}
		return map[string]interface{}{"engine": "voicevox", "speakers": speakers}
	default:
		return fetchProxiedVoices()
	}
}

func fetchProxiedVoices() any {
	ttsURL := config.Infra().TTSURL
	if ttsURL == "" {
		return fallbackVoices()
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(ttsURL + "/voices")
	if err != nil {
		slog.Warn("failed to fetch TTS voices, using fallback", "error", err)
		return fallbackVoices()
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != http.StatusOK {
		return fallbackVoices()
	}
	return json.RawMessage(body)
}

func fetchVoicevoxSpeakers() ([]SpeakerInfo, error) {
	voicevoxURL := config.Infra().VoicevoxURL
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(voicevoxURL + "/speakers")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var speakers []voicevoxSpeaker
	if err := json.NewDecoder(resp.Body).Decode(&speakers); err != nil {
		return nil, err
	}

	var result []SpeakerInfo
	for _, s := range speakers {
		for _, st := range s.Styles {
			result = append(result, SpeakerInfo{ID: st.ID, Name: s.Name, Style: st.Name})
		}
	}
	return result, nil
}

func fallbackVoices() map[string]interface{} {
	return map[string]interface{}{
		"engine":             "piper",
		"default_voice":      "tsukuyomi_mb",
		"default_speaker_id": 0,
		"voices": []map[string]interface{}{
			{
				"id":    "tsukuyomi_mb",
				"label": "つくよみちゃん MB版",
				"speakers": []map[string]interface{}{
					{"id": 0, "label": "デフォルト"},
				},
			},
		},
	}
}
