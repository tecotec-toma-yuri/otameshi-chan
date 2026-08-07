package chat

import (
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	maxRuneCount = 100
	flushTimeout = 1 * time.Second
)

// sentenceEnders contains Japanese punctuation that marks sentence boundaries.
var sentenceEnders = []string{"。", "、", "！", "？", "…"}

// FlushCallback is called when the buffer has a complete sentence to emit.
type FlushCallback func(text string)

// Buffer accumulates streaming text and flushes on sentence boundaries.
type Buffer struct {
	mu       sync.Mutex
	buf      strings.Builder
	callback FlushCallback
	timer    *time.Timer
	stopped  bool
}

// New creates a new Buffer with the given flush callback.
func New(callback FlushCallback) *Buffer {
	return &Buffer{
		callback: callback,
	}
}

// Add appends text to the buffer and flushes if a sentence boundary is found.
func (b *Buffer) Add(text string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.stopped {
		return
	}

	b.buf.WriteString(text)
	b.tryFlush()
	b.resetTimer()
}

// Flush forces any remaining text to be flushed.
func (b *Buffer) Flush() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.stopTimer()
	b.forceFlush()
}

// Stop stops the buffer and flushes remaining text.
func (b *Buffer) Stop() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.stopped = true
	b.stopTimer()
	b.forceFlush()
}

// Reset clears the buffer without flushing.
func (b *Buffer) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.stopTimer()
	b.buf.Reset()
}

func (b *Buffer) tryFlush() {
	content := b.buf.String()

	for {
		idx := b.findSentenceEnd(content)
		if idx < 0 {
			break
		}
		sentence := content[:idx]
		content = content[idx:]
		if sentence != "" {
			b.callback(sentence)
		}
	}

	if utf8.RuneCountInString(content) >= maxRuneCount {
		b.callback(content)
		content = ""
	}

	b.buf.Reset()
	b.buf.WriteString(content)
}

func (b *Buffer) findSentenceEnd(s string) int {
	bestIdx := -1
	for _, ender := range sentenceEnders {
		idx := strings.Index(s, ender)
		if idx >= 0 {
			end := idx + len(ender)
			if bestIdx < 0 || end < bestIdx {
				bestIdx = end
			}
		}
	}
	return bestIdx
}

func (b *Buffer) forceFlush() {
	content := b.buf.String()
	if content != "" {
		b.callback(content)
		b.buf.Reset()
	}
}

func (b *Buffer) resetTimer() {
	b.stopTimer()

	b.timer = time.AfterFunc(flushTimeout, func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		b.forceFlush()
	})
}

func (b *Buffer) stopTimer() {
	if b.timer != nil {
		b.timer.Stop()
		b.timer = nil
	}
}
