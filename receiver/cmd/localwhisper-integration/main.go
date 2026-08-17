package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"

	"localwhisper/receiver/internal/config"
	"localwhisper/receiver/internal/transcribe"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: localwhisper-integration WAV")
		os.Exit(2)
	}
	c, err := config.Load()
	if err != nil {
		slog.Error("configuration", "error", err)
		os.Exit(1)
	}
	text, duration, err := (transcribe.Runner{Binary: c.WhisperBin, Model: c.WhisperModel, Threads: c.WhisperThreads}).Run(context.Background(), os.Args[1])
	if err != nil {
		slog.Error("transcription", "error", err)
		os.Exit(1)
	}
	if err := os.MkdirAll("/tmp/localwhisper", 0o755); err != nil {
		slog.Error("create output directory", "error", err)
		os.Exit(1)
	}
	if err := os.WriteFile("/tmp/localwhisper/integration.txt", []byte(text+"\n"), 0o644); err != nil {
		slog.Error("write result", "error", err)
		os.Exit(1)
	}
	if err := exec.Command("wl-copy", text).Run(); err != nil {
		slog.Error("copy clipboard", "error", err)
		os.Exit(1)
	}
	fmt.Println(text)
	fmt.Fprintf(os.Stderr, "transcription duration: %s\n", duration)
}
