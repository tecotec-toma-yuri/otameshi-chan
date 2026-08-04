package session

import (
	"fmt"
	"sync"
)

// State represents the session state.
type State int

const (
	StateIdle       State = iota
	StateListening
	StateProcessing
	StateAISpeaking
	StateClosing
	StateClosed
)

// String returns the string representation of a State.
func (s State) String() string {
	switch s {
	case StateIdle:
		return "idle"
	case StateListening:
		return "listening"
	case StateProcessing:
		return "processing"
	case StateAISpeaking:
		return "ai_speaking"
	case StateClosing:
		return "closing"
	case StateClosed:
		return "closed"
	default:
		return "unknown"
	}
}

// validTransitions defines which state transitions are allowed.
var validTransitions = map[State][]State{
	StateIdle:       {StateListening, StateClosing, StateClosed},
	StateListening:  {StateProcessing, StateIdle, StateClosing, StateClosed},
	StateProcessing: {StateAISpeaking, StateListening, StateIdle, StateClosing, StateClosed},
	StateAISpeaking: {StateListening, StateIdle, StateProcessing, StateClosing, StateClosed},
	StateClosing:    {StateClosed},
	StateClosed:     {},
}

// StateMachine manages session state transitions with thread safety.
type StateMachine struct {
	mu    sync.RWMutex
	state State
}

// NewStateMachine creates a new StateMachine in StateIdle.
func NewStateMachine() *StateMachine {
	return &StateMachine{state: StateIdle}
}

// Current returns the current state.
func (sm *StateMachine) Current() State {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.state
}

// Transition attempts to move to the given state.
// Returns an error if the transition is not valid.
func (sm *StateMachine) Transition(to State) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	allowed := validTransitions[sm.state]
	for _, s := range allowed {
		if s == to {
			sm.state = to
			return nil
		}
	}
	return fmt.Errorf("invalid state transition from %s to %s", sm.state, to)
}
