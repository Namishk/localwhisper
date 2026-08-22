// Command localwhisper-overlay shows the desktop status pill.
//
//	serve        run the overlay window and its control server
//	set <state>  tell a running overlay to switch states
//
// States: recording, transcribing, copied, failed, disconnected.
package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/hajimehoshi/ebiten/v2"

	"localwhisper/receiver/internal/overlay"
)

const defaultAddr = "127.0.0.1:8767"

func main() {
	addr := os.Getenv("LOCALWHISPER_OVERLAY_ADDR")
	if addr == "" {
		addr = defaultAddr
	}
	switch args := os.Args[1:]; {
	case len(args) == 1 && args[0] == "serve":
		serve(addr)
	case len(args) == 2 && args[0] == "set" && overlay.Valid(args[1]):
		if err := overlay.Post(addr, args[1]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	default:
		fmt.Fprintln(os.Stderr, "usage: localwhisper-overlay serve | set <recording|transcribing|copied|failed|disconnected>")
		os.Exit(2)
	}
}

func serve(addr string) {
	store := overlay.NewStore()
	go func() {
		if err := overlay.Listen(addr, store); err != nil {
			slog.Error("overlay control server", "error", err)
			os.Exit(1)
		}
	}()

	ebiten.SetWindowTitle(windowTitle)
	ebiten.SetWindowDecorated(false)
	ebiten.SetWindowFloating(true)
	ebiten.SetWindowResizable(false)
	ebiten.SetInitFocused(false)
	ebiten.SetScreenTransparent(true)
	EnableClickThrough(windowTitle)
	if err := ebiten.RunGame(NewGame(store)); err != nil {
		slog.Error("overlay window", "error", err)
		os.Exit(1)
	}
}
