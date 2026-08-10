package stt

import "context"

type TranscribeResult struct {
	Text     string
	Language string
}

type STTService interface {
	Transcribe(ctx context.Context, audioData []byte) (TranscribeResult, error)
}
