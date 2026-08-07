package history

import "github.com/otameshi/backend/internal/service"

func GetHistory(id string) (*service.HistorySession, error) {
	return service.GetHistory(id)
}
