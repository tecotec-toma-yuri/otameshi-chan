package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/otameshi/backend/internal/openai"
)

func HandleModelsGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	models, err := openai.ListModels(r.Context())
	if err != nil {
		slog.Error("failed to list models", "error", err)
		http.Error(w, "failed to fetch models: "+err.Error(), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"models": models,
	})
}
