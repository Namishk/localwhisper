package transcribe

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type Runner struct {
	Binary  string
	Model   string
	Threads int
}

func (r Runner) Run(ctx context.Context, wavPath string) (string, time.Duration, error) {
	started := time.Now()
	args := []string{"-m", r.Model, "-f", wavPath, "-l", "auto", "-nt", "-np", "-t", fmt.Sprint(r.Threads)}
	command := exec.CommandContext(ctx, r.Binary, args...)
	output, err := command.Output()
	duration := time.Since(started)
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			return "", duration, fmt.Errorf("whisper exited: %w: %s", err, strings.TrimSpace(string(exitError.Stderr)))
		}
		return "", duration, fmt.Errorf("run whisper: %w", err)
	}
	text := strings.TrimSpace(string(output))
	if text == "" {
		return "", duration, fmt.Errorf("whisper returned no transcription")
	}
	return text, duration, nil
}
