package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const historyDataDir = "data/history"

type HistoryMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type HistorySession struct {
	ID        string           `json:"id"`
	StartedAt time.Time        `json:"started_at"`
	EndedAt   time.Time        `json:"ended_at"`
	Messages  []HistoryMessage `json:"messages"`
}

type HistorySessionSummary struct {
	ID        string    `json:"id"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`
	Turns     int       `json:"turns"`
	Preview   string    `json:"preview"`
}

func SaveHistory(sessionID string, startedAt time.Time, messages []HistoryMessage) error {
	if err := os.MkdirAll(historyDataDir, 0755); err != nil {
		return fmt.Errorf("create history dir: %w", err)
	}

	s := HistorySession{
		ID:        sessionID,
		StartedAt: startedAt,
		EndedAt:   time.Now(),
		Messages:  messages,
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	filename := fmt.Sprintf("%s_%s.json", s.StartedAt.Format("20060102_150405"), sessionID[:8])
	return os.WriteFile(filepath.Join(historyDataDir, filename), data, 0644)
}

func ListHistory() ([]HistorySessionSummary, error) {
	entries, err := os.ReadDir(historyDataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []HistorySessionSummary{}, nil
		}
		return nil, err
	}

	var result []HistorySessionSummary
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(historyDataDir, e.Name()))
		if err != nil {
			continue
		}
		var s HistorySession
		if err := json.Unmarshal(data, &s); err != nil {
			continue
		}

		preview := ""
		for _, m := range s.Messages {
			if m.Role == "user" {
				preview = m.Content
				break
			}
		}
		if len([]rune(preview)) > 50 {
			preview = string([]rune(preview)[:50]) + "..."
		}

		userTurns := 0
		for _, m := range s.Messages {
			if m.Role == "user" {
				userTurns++
			}
		}

		result = append(result, HistorySessionSummary{
			ID:        s.ID,
			StartedAt: s.StartedAt,
			EndedAt:   s.EndedAt,
			Turns:     userTurns,
			Preview:   preview,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].StartedAt.After(result[j].StartedAt)
	})

	return result, nil
}

func GetHistory(sessionID string) (*HistorySession, error) {
	entries, err := os.ReadDir(historyDataDir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !strings.Contains(e.Name(), sessionID[:8]) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(historyDataDir, e.Name()))
		if err != nil {
			return nil, err
		}
		var s HistorySession
		if err := json.Unmarshal(data, &s); err != nil {
			return nil, err
		}
		return &s, nil
	}
	return nil, fmt.Errorf("session not found: %s", sessionID)
}
