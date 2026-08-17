package config

import "testing"

func TestLoadRejectsNonLoopbackControlAddress(t *testing.T) {
	t.Setenv("CONTROL_ADDR", "0.0.0.0:8766")
	if _, err := Load(); err == nil {
		t.Fatal("Load() succeeded")
	}
}

func TestLoadUsesConfiguredThreadCount(t *testing.T) {
	t.Setenv("WHISPER_THREADS", "3")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.WhisperThreads != 3 {
		t.Fatalf("threads = %d, want 3", c.WhisperThreads)
	}
}
