package guardrail

import "context"

// GuardrailMonitor checks text for content policy violations.
type GuardrailMonitor interface {
	Check(ctx context.Context, text string) (passed bool, reason string, err error)
}
