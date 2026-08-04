package guardrail

import (
	"context"
	"strings"
	"time"
)

// StubMonitor is a guardrail monitor that always passes, except for test triggers.
type StubMonitor struct{}

// NewStubMonitor creates a new StubMonitor.
func NewStubMonitor() *StubMonitor {
	return &StubMonitor{}
}

// Check performs a content policy check. Always passes unless text contains "violation_test".
func (m *StubMonitor) Check(_ context.Context, text string) (bool, string, error) {
	// Simulate processing delay
	time.Sleep(50 * time.Millisecond)

	if strings.Contains(text, "violation_test") {
		return false, "content_policy_violation", nil
	}
	return true, "", nil
}
