package factory

import "github.com/otameshi/backend/internal/external/guardrail"

func NewGuardrailMonitor() guardrail.GuardrailMonitor {
	return guardrail.NewGuardrailMonitor()
}
