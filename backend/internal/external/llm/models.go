package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/otameshi/backend/internal/config"
)

type ModelInfo struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Preview bool   `json:"preview"`
}

type modelsResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

// Groq の Production モデル（ドキュメント準拠）。これ以外のチャットモデルは Preview 扱い。
var groqProductionModels = map[string]struct{}{
	"llama-3.1-8b-instant":   {},
	"llama-3.3-70b-versatile": {},
	"openai/gpt-oss-120b":    {},
	"openai/gpt-oss-20b":     {},
	"groq/compound":          {},
	"groq/compound-mini":     {},
}

// ListModels fetches available model IDs from the configured OpenAI-compatible /models API.
func ListModels(ctx context.Context) ([]ModelInfo, error) {
	infra := config.Infra()
	apiKey := infra.OpenAIAPIKey
	baseURL := infra.OpenAIBaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	baseURL = strings.TrimRight(baseURL, "/")

	if apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY is not set")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/models", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("models request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("models API returned %d: %s", resp.StatusCode, string(body))
	}

	var parsed modelsResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	useGroqRules := strings.Contains(strings.ToLower(baseURL), "groq.com")

	ids := make([]string, 0, len(parsed.Data))
	seen := make(map[string]struct{}, len(parsed.Data))
	for _, m := range parsed.Data {
		id := strings.TrimSpace(m.ID)
		if id == "" {
			continue
		}
		if !isSelectableChatModel(id) {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	sort.Strings(ids)

	out := make([]ModelInfo, 0, len(ids))
	for _, id := range ids {
		preview := isPreviewModel(id, useGroqRules)
		label := id
		if preview {
			label = id + " (Preview)"
		}
		out = append(out, ModelInfo{
			ID:      id,
			Label:   label,
			Preview: preview,
		})
	}
	return out, nil
}

func isPreviewModel(id string, useGroqRules bool) bool {
	lower := strings.ToLower(id)
	if strings.Contains(lower, "preview") {
		return true
	}
	if !useGroqRules {
		return false
	}
	if _, ok := groqProductionModels[id]; ok {
		return false
	}
	return true
}

func isSelectableChatModel(id string) bool {
	lower := strings.ToLower(id)
	exclude := []string{
		"whisper",
		"tts",
		"embedding",
		"moderation",
		"dall-e",
		"davinci",
		"babbage",
		"ada",
		"curie",
		"canary",
		"prompt-guard",
		"orpheus",
	}
	for _, e := range exclude {
		if strings.Contains(lower, e) {
			return false
		}
	}
	if strings.Contains(lower, "guard") && !strings.Contains(lower, "safeguard") {
		return false
	}
	return true
}
