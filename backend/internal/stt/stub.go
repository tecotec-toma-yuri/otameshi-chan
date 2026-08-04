package stt

import "context"

type StubSTT struct{}

func NewStubSTT() *StubSTT {
	return &StubSTT{}
}

func (s *StubSTT) Transcribe(_ context.Context, _ []byte) (TranscribeResult, error) {
	return TranscribeResult{Language: "ja"}, nil
}
