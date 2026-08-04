package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/otameshi/backend/internal/history"
	"github.com/otameshi/backend/internal/openai"
	"github.com/otameshi/backend/internal/protocol"
	"github.com/otameshi/backend/internal/session"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type WSHandler struct{}

func NewWSHandler() *WSHandler {
	return &WSHandler{}
}

func (h *WSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("WebSocket upgrade failed", "error", err)
		return
	}
	defer conn.Close()

	slog.Info("WebSocket connection established", "remote_addr", r.RemoteAddr)

	cfg := h.parseConfigFromQuery(r)
	mgr := session.NewManager(cfg)
	defer mgr.Close()

	if restoreID := r.URL.Query().Get("restore_session_id"); restoreID != "" {
		if hist, err := history.Get(restoreID); err == nil {
			msgs := make([]openai.ChatMessage, 0, len(hist.Messages))
			for _, m := range hist.Messages {
				msgs = append(msgs, openai.ChatMessage{Role: m.Role, Content: m.Content})
			}
			mgr.RestoreHistory(msgs)
			slog.Info("session restored from history", "restore_id", restoreID, "messages", len(msgs))
		} else {
			slog.Warn("failed to restore session history", "restore_id", restoreID, "error", err)
		}
	}

	mgr.Start()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for msg := range mgr.SendCh() {
			data, err := json.Marshal(msg)
			if err != nil {
				slog.Error("failed to marshal outbound message", "error", err)
				continue
			}
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				slog.Error("failed to write WebSocket message", "error", err)
				return
			}
		}
	}()

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure, websocket.CloseNoStatusReceived) {
				slog.Error("WebSocket read error", "error", err)
			} else {
				slog.Info("WebSocket connection closed", "remote_addr", r.RemoteAddr)
			}
			return
		}

		var msg protocol.InboundMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			slog.Warn("failed to parse incoming message", "error", err, "data", string(data))
			errMsg := protocol.NewOutbound(protocol.TypeError, protocol.Error{
				Code:        "parse_error",
				Message:     "Invalid JSON message",
				Recoverable: true,
			})
			errData, _ := json.Marshal(errMsg)
			conn.WriteMessage(websocket.TextMessage, errData)
			continue
		}

		mgr.HandleMessage(msg)
	}
}

func (h *WSHandler) parseConfigFromQuery(r *http.Request) protocol.SessionConfig {
	config := protocol.SessionConfig{
		RecommendationMode:         "ai_driven",
		PostRecommendationBehavior: "return_to_conversation",
	}

	if mode := r.URL.Query().Get("recommendation_mode"); mode != "" {
		config.RecommendationMode = mode
	}
	if behavior := r.URL.Query().Get("post_recommendation_behavior"); behavior != "" {
		config.PostRecommendationBehavior = behavior
	}

	return config
}
