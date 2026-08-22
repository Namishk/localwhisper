package config

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
)

type Config struct {
	WhisperBin     string
	WhisperModel   string
	WhisperThreads int
	IndicatorBin   string
	WSAddr         string
	ControlAddr    string
	Token          string
	WAVPath        string
}

func Load() (Config, error) {
	root := checkoutRoot()
	c := platformDefaults(root)
	c.WhisperModel = env("WHISPER_MODEL", filepath.Join(root, "whisper.cpp/models/ggml-large-v3-turbo-q5_0.bin"))
	c.WhisperThreads = 8
	c.WSAddr = env("WS_ADDR", ":8765")
	c.ControlAddr = env("CONTROL_ADDR", "127.0.0.1:8766")
	c.Token = os.Getenv("LOCALWHISPER_TOKEN")
	if value := os.Getenv("WHISPER_BIN"); value != "" {
		c.WhisperBin = value
	}
	if value := os.Getenv("LOCALWHISPER_INDICATOR"); value != "" {
		c.IndicatorBin = value
	}
	if value := os.Getenv("LOCALWHISPER_WAV"); value != "" {
		c.WAVPath = value
	}
	if value := os.Getenv("WHISPER_THREADS"); value != "" {
		threads, err := strconv.Atoi(value)
		if err != nil || threads < 1 {
			return Config{}, fmt.Errorf("WHISPER_THREADS must be a positive integer")
		}
		c.WhisperThreads = threads
	}
	if err := validateAddr(c.WSAddr, false); err != nil {
		return Config{}, fmt.Errorf("WS_ADDR: %w", err)
	}
	if err := validateAddr(c.ControlAddr, true); err != nil {
		return Config{}, fmt.Errorf("CONTROL_ADDR: %w", err)
	}
	return c, nil
}

func checkoutRoot() string {
	if root := os.Getenv("LOCALWHISPER_HOME"); root != "" {
		return root
	}
	if workingDirectory, err := os.Getwd(); err == nil {
		if root, ok := findRoot(workingDirectory); ok {
			return root
		}
	}
	if executable, err := os.Executable(); err == nil {
		if root, ok := findRoot(filepath.Dir(executable)); ok {
			return root
		}
	}
	return "."
}

func findRoot(start string) (string, bool) {
	directory := start
	for range 4 {
		if _, err := os.Stat(filepath.Join(directory, "whisper.cpp")); err == nil {
			return directory, true
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			break
		}
		directory = parent
	}
	return "", false
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func validateAddr(addr string, mustBeLoopback bool) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	if mustBeLoopback && host != "127.0.0.1" && host != "::1" && host != "localhost" {
		return fmt.Errorf("must bind to loopback")
	}
	return nil
}
