package ttsVoices

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/otameshi/backend/internal/config"
)

type VoicesResult struct {
	Body []byte
	Err  error
}

func FetchTTSVoices(ctx context.Context) ([]byte, error) {
	ttsURL := config.Get().TTSURL
	if ttsURL == "" {
		return nil, nil
	}
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ttsURL+"/voices", nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		slog.Warn("failed to fetch TTS voices", "error", err)
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
