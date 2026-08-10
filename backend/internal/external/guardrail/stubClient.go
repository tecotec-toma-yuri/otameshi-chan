package guardrail

import (
	"context"
	"strings"
	"time"
)

// StubGuardrailMonitor is a guardrail monitor that always passes, except for test triggers.
type StubGuardrailMonitor struct{}

func NewStubGuardrailMonitor() *StubGuardrailMonitor {
	return &StubGuardrailMonitor{}
}

// Check performs a content policy check. Always passes unless text contains "violation_test".
func (m *StubGuardrailMonitor) Check(_ context.Context, text string) (bool, string, error) {
	time.Sleep(50 * time.Millisecond)

	if strings.Contains(text, "violation_test") {
		return false, "content_policy_violation", nil
	}
	return true, "", nil
}
