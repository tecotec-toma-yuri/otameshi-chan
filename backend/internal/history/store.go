package history

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const dataDir = "data/history"

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Session struct {
	ID        string    `json:"id"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`
	Messages  []Message `json:"messages"`
}

type SessionSummary struct {
	ID        string    `json:"id"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`
	Turns     int       `json:"turns"`
	Preview   string    `json:"preview"`
}

func Save(sessionID string, startedAt time.Time, messages []Message) error {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("create history dir: %w", err)
	}

	s := Session{
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
	return os.WriteFile(filepath.Join(dataDir, filename), data, 0644)
}

func List() ([]SessionSummary, error) {
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []SessionSummary{}, nil
		}
		return nil, err
	}

	var result []SessionSummary
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dataDir, e.Name()))
		if err != nil {
			continue
		}
		var s Session
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

		result = append(result, SessionSummary{
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

func Get(sessionID string) (*Session, error) {
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !strings.Contains(e.Name(), sessionID[:8]) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dataDir, e.Name()))
		if err != nil {
			return nil, err
		}
		var s Session
		if err := json.Unmarshal(data, &s); err != nil {
			return nil, err
		}
		return &s, nil
	}
	return nil, fmt.Errorf("session not found: %s", sessionID)
}
