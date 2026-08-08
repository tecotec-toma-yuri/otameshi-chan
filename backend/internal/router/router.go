package router

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/otameshi/backend/internal/config"
	"github.com/otameshi/backend/internal/handler/rest"
	"github.com/otameshi/backend/internal/handler/ws"
)

func New(wsHandler *ws.Handler) chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{config.Get().CORSOrigin},
		AllowedMethods: []string{"GET", "PUT", "POST", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type", "Authorization"},
	}))
	r.Handle("/ws", wsHandler)
	r.Get("/health", rest.HandleHealth)
	r.Route("/api", func(r chi.Router) {
		r.Get("/config", rest.HandleConfigGet)
		r.Put("/config", rest.HandleConfigPut)
		r.Get("/models", rest.HandleModelsGet)
		r.Get("/tts/voices", rest.HandleTTSVoicesGet)
		r.Get("/history", rest.HandleHistoryList)
		r.Get("/history/{id}", rest.HandleHistoryGet)
	})
	return r
}
