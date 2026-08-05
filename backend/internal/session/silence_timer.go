package session

import (
	"sync"
	"time"
)

const (
	silenceStage1Duration = 60 * time.Second
	silenceStage2Duration = 30 * time.Second
)

// SilenceTimer implements a 2-stage silence detection timer.
// Stage 1: fires after 60s of silence with a confirmation prompt callback.
// Stage 2: fires after an additional 30s with a session close callback.
type SilenceTimer struct {
	mu              sync.Mutex
	timer           *time.Timer
	stage           int // 0=not started, 1=stage1 running, 2=stage2 running
	paused          bool
	remaining       time.Duration
	pausedAt        time.Time
	stageStartedAt  time.Time
	onConfirmation  func()
	onClose         func()
}

// NewSilenceTimer creates a new SilenceTimer with the given callbacks.
func NewSilenceTimer(onConfirmation, onClose func()) *SilenceTimer {
	return &SilenceTimer{
		onConfirmation: onConfirmation,
		onClose:        onClose,
	}
}

// Start begins the stage 1 timer.
func (st *SilenceTimer) Start() {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.startStage1()
}

// Reset restarts the timer from stage 1.
func (st *SilenceTimer) Reset() {
	st.mu.Lock()
	defer st.mu.Unlock()

	st.stopTimer()
	st.paused = false
	st.startStage1()
}

// Pause pauses the current timer (e.g., during AI speaking).
func (st *SilenceTimer) Pause() {
	st.mu.Lock()
	defer st.mu.Unlock()

	if st.paused || st.stage == 0 {
		return
	}

	st.paused = true
	st.pausedAt = time.Now()

	elapsed := time.Since(st.stageStartedAt)
	var totalDuration time.Duration
	if st.stage == 1 {
		totalDuration = silenceStage1Duration
	} else {
		totalDuration = silenceStage2Duration
	}
	st.remaining = totalDuration - elapsed
	if st.remaining < 0 {
		st.remaining = 0
	}

	st.stopTimer()
}

// Resume resumes a paused timer with the remaining duration.
func (st *SilenceTimer) Resume() {
	st.mu.Lock()
	defer st.mu.Unlock()

	if !st.paused || st.stage == 0 {
		return
	}

	st.paused = false
	st.stageStartedAt = time.Now()

	currentStage := st.stage
	st.timer = time.AfterFunc(st.remaining, func() {
		if currentStage == 1 {
			st.handleStage1Expired()
		} else {
			st.handleStage2Expired()
		}
	})
}

// Stop stops the timer completely.
func (st *SilenceTimer) Stop() {
	st.mu.Lock()
	defer st.mu.Unlock()

	st.stopTimer()
	st.stage = 0
	st.paused = false
}

func (st *SilenceTimer) startStage1() {
	st.stage = 1
	st.stageStartedAt = time.Now()
	st.timer = time.AfterFunc(silenceStage1Duration, st.handleStage1Expired)
}

func (st *SilenceTimer) handleStage1Expired() {
	st.mu.Lock()
	if st.stage != 1 || st.paused {
		st.mu.Unlock()
		return
	}
	st.stage = 2
	st.stageStartedAt = time.Now()
	st.timer = time.AfterFunc(silenceStage2Duration, st.handleStage2Expired)
	st.mu.Unlock()

	if st.onConfirmation != nil {
		st.onConfirmation()
	}
}

func (st *SilenceTimer) handleStage2Expired() {
	st.mu.Lock()
	if st.stage != 2 || st.paused {
		st.mu.Unlock()
		return
	}
	st.mu.Unlock()

	if st.onClose != nil {
		st.onClose()
	}
}

func (st *SilenceTimer) stopTimer() {
	if st.timer != nil {
		st.timer.Stop()
		st.timer = nil
	}
}
