package ws

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/otameshi/backend/internal/config"
	"github.com/otameshi/backend/internal/external/llm"
	"github.com/otameshi/backend/internal/factory"
	"github.com/otameshi/backend/internal/protocol"
	"github.com/otameshi/backend/internal/service"
	"github.com/otameshi/backend/internal/usecase/chat"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		return origin == "" || strings.HasPrefix(origin, config.Infra().CORSOrigin)
	},
}

type Handler struct {
	svc *factory.ChatDependencies
}

func NewHandler(svc *factory.ChatDependencies) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("WebSocket upgrade failed", "error", err)
		return
	}
	defer conn.Close()

	slog.Info("WebSocket connection established", "remote_addr", r.RemoteAddr)

	conn.SetReadDeadline(time.Now().Add(90 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		return nil
	})

	cfg := parseConfigFromQuery(r)
	mgr := chat.NewManager(cfg, h.svc.NewLLMClient(), h.svc.NewTTSService(), h.svc.NewSTTService(), h.svc.NewGuardrailMonitor())
	defer mgr.Close()

	if restoreID := r.URL.Query().Get("restore_session_id"); restoreID != "" {
		if hist, err := service.GetHistory(restoreID); err == nil {
			msgs := make([]llm.ChatMessage, 0, len(hist.Messages))
			for _, m := range hist.Messages {
				msgs = append(msgs, llm.ChatMessage{Role: m.Role, Content: m.Content})
			}
			mgr.RestoreHistory(msgs)
			slog.Info("session restored from history", "restore_id", restoreID, "messages", len(msgs))
		} else {
			slog.Warn("failed to restore session history", "restore_id", restoreID, "error", err)
		}
	}

	mgr.Start()

	done := make(chan struct{})
	pingTicker := time.NewTicker(30 * time.Second)
	go func() {
		defer close(done)
		defer pingTicker.Stop()
		for {
			select {
			case msg, ok := <-mgr.SendCh():
				if !ok {
					return
				}
				data, err := json.Marshal(msg)
				if err != nil {
					slog.Error("failed to marshal outbound message", "error", err)
					continue
				}
				if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
					slog.Error("failed to write WebSocket message", "error", err)
					return
				}
			case <-pingTicker.C:
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
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
			mgr.Close()
			<-done
			return
		}

		var msg protocol.InboundMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			slog.Warn("failed to parse incoming message", "error", err, "data", string(data))
			mgr.SendError("parse_error", "Invalid JSON message")
			continue
		}

		mgr.HandleMessage(msg)
	}
}

func parseConfigFromQuery(r *http.Request) protocol.SessionConfig {
	cfg := protocol.SessionConfig{
		RecommendationMode:         "ai_driven",
		PostRecommendationBehavior: "return_to_conversation",
	}
	if mode := r.URL.Query().Get("recommendation_mode"); mode != "" {
		cfg.RecommendationMode = mode
	}
	if behavior := r.URL.Query().Get("post_recommendation_behavior"); behavior != "" {
		cfg.PostRecommendationBehavior = behavior
	}
	return cfg
}
