package models

import (
	"context"

	"github.com/otameshi/backend/internal/external/llm"
)

func ListModels(ctx context.Context) ([]llm.ModelInfo, error) {
	return llm.ListModels(ctx)
}
