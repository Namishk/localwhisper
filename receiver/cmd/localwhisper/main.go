package main

import (
	"log/slog"
	"os"

	"localwhisper/receiver/internal/config"
	"localwhisper/receiver/internal/server"
)

func main() {
	c, err := config.Load()
	if err != nil {
		slog.Error("invalid configuration", "error", err)
		os.Exit(1)
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	log.Info("starting LocalWhisper", "ws_addr", c.WSAddr, "control_addr", c.ControlAddr)
	if err := server.New(c, log).ListenAndServe(); err != nil {
		slog.Error("receiver stopped", "error", err)
		os.Exit(1)
	}
}
