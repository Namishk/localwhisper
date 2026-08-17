package audio

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteWAVProducesPCMHeader(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audio.wav")
	pcm := []byte{1, 0, 2, 0}
	if err := WriteWAV(path, pcm); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		t.Fatalf("invalid container: %q", data[:12])
	}
	if string(data[12:16]) != "fmt " {
		t.Fatalf("format chunk = %q, want fmt", data[12:16])
	}
	if binary.LittleEndian.Uint32(data[24:28]) != SampleRate {
		t.Fatal("wrong sample rate")
	}
	if binary.LittleEndian.Uint16(data[34:36]) != BitsPerSample {
		t.Fatal("wrong bit depth")
	}
	if binary.LittleEndian.Uint32(data[40:44]) != uint32(len(pcm)) {
		t.Fatal("wrong data size")
	}
	if string(data[44:]) != string(pcm) {
		t.Fatal("PCM changed")
	}
}

func TestWriteWAVRejectsOddPCM(t *testing.T) {
	if err := WriteWAV(filepath.Join(t.TempDir(), "audio.wav"), []byte{1}); err == nil {
		t.Fatal("WriteWAV() succeeded")
	}
}
