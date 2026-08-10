package history

import "github.com/otameshi/backend/internal/service"

func ListHistory() ([]service.HistorySessionSummary, error) {
	return service.ListHistory()
}
