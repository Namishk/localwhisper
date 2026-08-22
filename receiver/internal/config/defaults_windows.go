package config

import "path/filepath"

// platformDefaults returns OS-specific fallbacks. root is the checkout or
// install root containing whisper.cpp.
func platformDefaults(root string) Config {
	return Config{
		WhisperBin:   filepath.Join(root, "whisper.cpp/build/bin/Release/whisper-cli.exe"),
		IndicatorBin: "",
		WAVPath:      filepath.Join(root, "latest.wav"),
	}
}
