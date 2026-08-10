package rest

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/otameshi/backend/internal/config"
	usecaseconfig "github.com/otameshi/backend/internal/usecase/config"
	usecasehistory "github.com/otameshi/backend/internal/usecase/history"
	usecasemodels "github.com/otameshi/backend/internal/usecase/models"
	usecasettsvoices "github.com/otameshi/backend/internal/usecase/ttsVoices"
)

func HandleHealth(w http.ResponseWriter, r *http.Request) {
	renderJSON(w, r, http.StatusOK, map[string]string{"status": "ok"})
}

// --- config ---

func HandleConfigGet(w http.ResponseWriter, r *http.Request) {
	renderJSON(w, r, http.StatusOK, usecaseconfig.GetConfig())
}

func HandleConfigPut(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		renderError(w, r, http.StatusBadRequest, errors.New("failed to read body"))
		return
	}
	defer r.Body.Close()

	var c config.Config
	if err := json.Unmarshal(body, &c); err != nil {
		renderError(w, r, http.StatusBadRequest, errors.New("invalid JSON"))
		return
	}
	if err := usecaseconfig.UpdateConfig(c); err != nil {
		renderError(w, r, http.StatusInternalServerError, errors.New("failed to save config"))
		return
	}
	renderJSON(w, r, http.StatusOK, usecaseconfig.GetConfig())
}

// --- models ---

func HandleModelsGet(w http.ResponseWriter, r *http.Request) {
	models, err := usecasemodels.ListModels(r.Context())
	if err != nil {
		slog.Error("failed to list models", "error", err)
		renderError(w, r, http.StatusBadGateway, errors.New("failed to fetch models: "+err.Error()))
		return
	}
	renderJSON(w, r, http.StatusOK, map[string]interface{}{"models": models})
}

// --- TTS voices ---

func HandleTTSVoicesGet(w http.ResponseWriter, r *http.Request) {
	engine := r.URL.Query().Get("engine")
	renderJSON(w, r, http.StatusOK, usecasettsvoices.GetVoices(engine))
}

// --- history ---

func HandleHistoryList(w http.ResponseWriter, r *http.Request) {
	sessions, err := usecasehistory.ListHistory()
	if err != nil {
		renderError(w, r, http.StatusInternalServerError, err)
		return
	}
	renderJSON(w, r, http.StatusOK, sessions)
}

func HandleHistoryGet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		renderError(w, r, http.StatusBadRequest, errors.New("invalid session id"))
		return
	}
	session, err := usecasehistory.GetHistory(id)
	if err != nil {
		renderError(w, r, http.StatusNotFound, err)
		return
	}
	renderJSON(w, r, http.StatusOK, session)
}
