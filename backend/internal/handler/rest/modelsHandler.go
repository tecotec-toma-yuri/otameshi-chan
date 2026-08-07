package rest

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/otameshi/backend/internal/external/llm"
)

func HandleModelsGet(w http.ResponseWriter, r *http.Request) {
	models, err := llm.ListModels(r.Context())
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
