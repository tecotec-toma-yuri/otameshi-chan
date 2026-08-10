package tts

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"time"
)

// WavToOpus converts WAV audio data to Ogg/Opus using ffmpeg via stdin/stdout pipe.
func WavToOpus(ctx context.Context, wav []byte) ([]byte, error) {
	started := time.Now()

	// Use background context so that a cancelled parent doesn't kill ffmpeg mid-encode.
	// The encode is fast (< 50ms) and we always want the result if TTS already completed.
	cmd := exec.CommandContext(context.Background(), "ffmpeg",
		"-hide_banner",
		"-loglevel", "error",
		"-f", "wav",
		"-i", "pipe:0",
		"-c:a", "libopus",
		"-b:a", "24000",
		"-vbr", "on",
		"-application", "voip",
		"-f", "ogg",
		"pipe:1",
	)
	cmd.Stdin = bytes.NewReader(wav)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		stderrStr := stderr.String()
		if len(stderrStr) > 500 {
			stderrStr = stderrStr[:500]
		}
		slog.Error("ffmpeg opus encode failed",
			"error", err,
			"stderr", stderrStr,
			"wav_bytes", len(wav),
		)
		return nil, fmt.Errorf("ffmpeg opus encode: %w", err)
	}

	opus := stdout.Bytes()
	slog.Debug("WAV→Opus",
		"wav_bytes", len(wav),
		"opus_bytes", len(opus),
		"ratio", fmt.Sprintf("%.1fx", float64(len(wav))/float64(len(opus))),
		"encode_ms", time.Since(started).Milliseconds(),
	)
	return opus, nil
}
