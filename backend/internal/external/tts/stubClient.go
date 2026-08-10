package tts

import (
	"bytes"
	"context"
	"encoding/binary"
	"unicode/utf8"
)

// StubTTS generates valid WAV audio with duration proportional to text length.
type StubTTS struct{}

// NewStubTTS creates a new StubTTS.
func NewStubTTS() *StubTTS {
	return &StubTTS{}
}

// Synthesize generates a valid WAV file (16kHz, mono, 16-bit PCM) with silence.
// Duration is ~80ms per rune, capped at 10 seconds.
func (s *StubTTS) SynthesizeWithLang(ctx context.Context, text string, _ string) ([]byte, error) {
	return s.Synthesize(ctx, text)
}

func (s *StubTTS) Synthesize(_ context.Context, text string) ([]byte, error) {
	const sampleRate = 16000
	const bitsPerSample = 16
	const numChannels = 1
	const maxDurationMs = 10000

	runeCount := utf8.RuneCountInString(text)
	durationMs := runeCount * 80
	if durationMs > maxDurationMs {
		durationMs = maxDurationMs
	}
	if durationMs < 100 {
		durationMs = 100
	}

	numSamples := sampleRate * durationMs / 1000
	dataSize := numSamples * numChannels * (bitsPerSample / 8)

	var buf bytes.Buffer

	// RIFF header
	buf.WriteString("RIFF")
	binary.Write(&buf, binary.LittleEndian, uint32(36+dataSize))
	buf.WriteString("WAVE")

	// fmt sub-chunk
	buf.WriteString("fmt ")
	binary.Write(&buf, binary.LittleEndian, uint32(16))
	binary.Write(&buf, binary.LittleEndian, uint16(1))
	binary.Write(&buf, binary.LittleEndian, uint16(numChannels))
	binary.Write(&buf, binary.LittleEndian, uint32(sampleRate))
	binary.Write(&buf, binary.LittleEndian, uint32(sampleRate*numChannels*(bitsPerSample/8)))
	binary.Write(&buf, binary.LittleEndian, uint16(numChannels*(bitsPerSample/8)))
	binary.Write(&buf, binary.LittleEndian, uint16(bitsPerSample))

	// data sub-chunk
	buf.WriteString("data")
	binary.Write(&buf, binary.LittleEndian, uint32(dataSize))

	silence := make([]byte, dataSize)
	buf.Write(silence)

	return buf.Bytes(), nil
}
