package handler

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/otameshi/backend/internal/config"
)

func HandleTTSVoicesGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	engine := r.URL.Query().Get("engine")
	if engine == "" {
		cfg := config.Get()
		engine = cfg.TTSEngine
	}
	if engine == "" {
		engine = os.Getenv("TTS_ENGINE")
	}
	w.Header().Set("Content-Type", "application/json")

	switch engine {
	case "voicevox":
		speakers, err := fetchVoicevoxSpeakers()
		if err != nil {
			slog.Warn("failed to fetch VOICEVOX speakers", "error", err)
			json.NewEncoder(w).Encode(map[string]interface{}{"engine": "voicevox", "speakers": []interface{}{}})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"engine": "voicevox", "speakers": speakers})
	default:
		ttsURL := os.Getenv("TTS_URL")
		if ttsURL == "" {
			json.NewEncoder(w).Encode(fallbackVoices())
			return
		}
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Get(ttsURL + "/voices")
		if err != nil {
			slog.Warn("failed to fetch TTS voices, using fallback", "error", err)
			json.NewEncoder(w).Encode(fallbackVoices())
			return
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil || resp.StatusCode != http.StatusOK {
			json.NewEncoder(w).Encode(fallbackVoices())
			return
		}
		w.Write(body)
	}
}

type voicevoxSpeaker struct {
	Name   string              `json:"name"`
	UUID   string              `json:"speaker_uuid"`
	Styles []voicevoxStyle     `json:"styles"`
}

type voicevoxStyle struct {
	Name string `json:"name"`
	ID   int    `json:"id"`
}

type speakerInfo struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Style string `json:"style"`
}

func fetchVoicevoxSpeakers() ([]speakerInfo, error) {
	voicevoxURL := os.Getenv("VOICEVOX_URL")
	if voicevoxURL == "" {
		voicevoxURL = "http://voicevox:50021"
	}

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

	var result []speakerInfo
	for _, s := range speakers {
		for _, st := range s.Styles {
			result = append(result, speakerInfo{
				ID:    st.ID,
				Name:  s.Name,
				Style: st.Name,
			})
		}
	}
	return result, nil
}

func fallbackVoices() map[string]interface{} {
	return map[string]interface{}{
		"engine":           "piper",
		"default_voice":    "tsukuyomi_mb",
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
