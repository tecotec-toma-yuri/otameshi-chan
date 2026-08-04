package tts

import "context"

// TTSService converts text to speech audio.
type TTSService interface {
	Synthesize(ctx context.Context, text string) ([]byte, error)
	SynthesizeWithLang(ctx context.Context, text string, language string) ([]byte, error)
}
