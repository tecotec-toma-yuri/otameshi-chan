package guardrail

import "context"

// Monitor checks text for content policy violations.
type Monitor interface {
	Check(ctx context.Context, text string) (passed bool, reason string, err error)
}
