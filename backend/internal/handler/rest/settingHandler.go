package rest

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/otameshi/backend/internal/config"
	"github.com/otameshi/backend/internal/external/llm"
	"github.com/otameshi/backend/internal/handler"
	"github.com/otameshi/backend/internal/service"
)

func HandleHealth(w http.ResponseWriter, r *http.Request) {
	handler.JSON(w, r, http.StatusOK, map[string]string{"status": "ok"})
}

// --- config ---

func HandleConfigGet(w http.ResponseWriter, r *http.Request) {
	handler.JSON(w, r, http.StatusOK, config.Get())
}

func HandleConfigPut(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		handler.Error(w, r, http.StatusBadRequest, errors.New("failed to read body"))
		return
	}
	defer r.Body.Close()

	var c config.Config
	if err := json.Unmarshal(body, &c); err != nil {
		handler.Error(w, r, http.StatusBadRequest, errors.New("invalid JSON"))
		return
	}
	if err := config.Update(c); err != nil {
		handler.Error(w, r, http.StatusInternalServerError, errors.New("failed to save config"))
		return
	}
	handler.JSON(w, r, http.StatusOK, config.Get())
}

// --- models ---

func HandleModelsGet(w http.ResponseWriter, r *http.Request) {
	models, err := llm.ListModels(r.Context())
	if err != nil {
		slog.Error("failed to list models", "error", err)
		handler.Error(w, r, http.StatusBadGateway, errors.New("failed to fetch models: "+err.Error()))
		return
	}
	handler.JSON(w, r, http.StatusOK, map[string]interface{}{"models": models})
}

// --- TTS voices ---

func HandleTTSVoicesGet(w http.ResponseWriter, r *http.Request) {
	engine := r.URL.Query().Get("engine")
	if engine == "" {
		engine = config.Get().TTSEngine
	}
	switch engine {
	case "voicevox":
		speakers, err := fetchVoicevoxSpeakers()
		if err != nil {
			slog.Warn("failed to fetch VOICEVOX speakers", "error", err)
			handler.JSON(w, r, http.StatusOK, map[string]interface{}{"engine": "voicevox", "speakers": []interface{}{}})
			return
		}
		handler.JSON(w, r, http.StatusOK, map[string]interface{}{"engine": "voicevox", "speakers": speakers})
	default:
		ttsURL := config.Get().TTSURL
		if ttsURL == "" {
			handler.JSON(w, r, http.StatusOK, fallbackVoices())
			return
		}
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Get(ttsURL + "/voices")
		if err != nil {
			slog.Warn("failed to fetch TTS voices, using fallback", "error", err)
			handler.JSON(w, r, http.StatusOK, fallbackVoices())
			return
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil || resp.StatusCode != http.StatusOK {
			handler.JSON(w, r, http.StatusOK, fallbackVoices())
			return
		}
		handler.JSON(w, r, http.StatusOK, json.RawMessage(body))
	}
}

type voicevoxSpeaker struct {
	Name   string          `json:"name"`
	UUID   string          `json:"speaker_uuid"`
	Styles []voicevoxStyle `json:"styles"`
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
	voicevoxURL := config.Get().VoicevoxURL
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
			result = append(result, speakerInfo{ID: st.ID, Name: s.Name, Style: st.Name})
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

// --- history ---

func HandleHistoryList(w http.ResponseWriter, r *http.Request) {
	sessions, err := service.ListHistory()
	if err != nil {
		handler.Error(w, r, http.StatusInternalServerError, err)
		return
	}
	handler.JSON(w, r, http.StatusOK, sessions)
}

func HandleHistoryGet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		handler.Error(w, r, http.StatusBadRequest, errors.New("invalid session id"))
		return
	}
	session, err := service.GetHistory(id)
	if err != nil {
		handler.Error(w, r, http.StatusNotFound, err)
		return
	}
	handler.JSON(w, r, http.StatusOK, session)
}
