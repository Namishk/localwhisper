package transcribe

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRunReturnsOnlyStandardOutput(t *testing.T) {
	script := filepath.Join(t.TempDir(), "whisper")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho diagnostic >&2\nprintf 'hello world\\n'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	text, _, err := (Runner{Binary: script, Model: "model", Threads: 1}).Run(context.Background(), "audio.wav")
	if err != nil {
		t.Fatal(err)
	}
	if text != "hello world" {
		t.Fatalf("text = %q", text)
	}
}
