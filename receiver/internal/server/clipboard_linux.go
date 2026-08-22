//go:build linux

package server

import "os/exec"

func copyClipboard(text string) error { return exec.Command("wl-copy", text).Run() }
