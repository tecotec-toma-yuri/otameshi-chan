package ttsVoices

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"
)

type VoicesResult struct {
	Body []byte
	Err  error
}

func FetchTTSVoices(ctx context.Context) ([]byte, error) {
	ttsURL := os.Getenv("TTS_URL")
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
