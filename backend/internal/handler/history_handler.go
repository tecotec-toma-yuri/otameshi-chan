package handler

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"github.com/otameshi/backend/internal/history"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func HandleHistoryList(w http.ResponseWriter, r *http.Request) {
	sessions, err := history.List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sessions)
}

func HandleHistoryGet(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/history/")
	if id == "" || !uuidPattern.MatchString(id) {
		http.Error(w, "invalid session id", http.StatusBadRequest)
		return
	}
	session, err := history.Get(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(session)
}
