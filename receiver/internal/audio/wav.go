package audio

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
)

const SampleRate = 16000
const Channels = 1
const BitsPerSample = 16

func WriteWAV(path string, pcm []byte) error {
	if len(pcm)%2 != 0 {
		return fmt.Errorf("PCM byte count must be even")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create WAV directory: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create WAV: %w", err)
	}
	defer f.Close()
	header := make([]byte, 44)
	copy(header[0:4], "RIFF")
	binary.LittleEndian.PutUint32(header[4:8], uint32(36+len(pcm)))
	copy(header[8:12], "WAVE")
	copy(header[12:16], "fmt ")
	binary.LittleEndian.PutUint32(header[16:20], 16)
	binary.LittleEndian.PutUint16(header[20:22], 1)
	binary.LittleEndian.PutUint16(header[22:24], Channels)
	binary.LittleEndian.PutUint32(header[24:28], SampleRate)
	byteRate := SampleRate * Channels * BitsPerSample / 8
	binary.LittleEndian.PutUint32(header[28:32], uint32(byteRate))
	binary.LittleEndian.PutUint16(header[32:34], Channels*BitsPerSample/8)
	binary.LittleEndian.PutUint16(header[34:36], BitsPerSample)
	copy(header[36:40], "data")
	binary.LittleEndian.PutUint32(header[40:44], uint32(len(pcm)))
	if _, err := f.Write(header); err != nil {
		return fmt.Errorf("write WAV header: %w", err)
	}
	if _, err := f.Write(pcm); err != nil {
		return fmt.Errorf("write WAV samples: %w", err)
	}
	return nil
}

func DurationMilliseconds(pcmBytes int) int64 {
	return int64(pcmBytes) * 1000 / (SampleRate * Channels * BitsPerSample / 8)
}
