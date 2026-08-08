package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/otameshi/backend/internal/config"
	wshandler "github.com/otameshi/backend/internal/handler/ws"
	"github.com/otameshi/backend/internal/router"
)

func main() {
	logCloser, err := config.SetupLogging()
	if err != nil {
		slog.Error("failed to setup file logging, using stdout only", "error", err)
	}
	if logCloser != nil {
		defer logCloser.Close()
	}

	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "data/config.yaml"
	}
	config.Load(configPath)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	wsHandler := wshandler.NewHandler(newLLMClient, newTTSService, newSTTService, newGuardrailMonitor)
	r := router.New(wsHandler)

	srv := &http.Server{
		Addr:        ":" + port,
		Handler:     r,
		IdleTimeout: 120 * time.Second,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		slog.Info("server starting", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-quit
	slog.Info("shutting down server")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("server shutdown error", "error", err)
	}

	slog.Info("server stopped")
}
