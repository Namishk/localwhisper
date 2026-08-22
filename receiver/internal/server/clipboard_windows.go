//go:build windows

package server

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os/exec"
	"unicode/utf16"
)

// copyClipboard pipes UTF-16LE text to clip.exe, which places it on the
// Windows clipboard. UTF-16LE is required for non-ASCII text.
func copyClipboard(text string) error {
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xFE}) // UTF-16LE BOM
	for _, unit := range utf16.Encode([]rune(text)) {
		binary.Write(&buf, binary.LittleEndian, unit)
	}
	cmd := exec.Command("clip")
	cmd.Stdin = &buf
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run clip: %w", err)
	}
	return nil
}
