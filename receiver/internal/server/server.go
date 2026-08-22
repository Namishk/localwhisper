package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os/exec"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"localwhisper/receiver/internal/audio"
	"localwhisper/receiver/internal/config"
	"localwhisper/receiver/internal/protocol"
	"localwhisper/receiver/internal/state"
	"localwhisper/receiver/internal/transcribe"
)

type Server struct {
	config    config.Config
	log       *slog.Logger
	runner    transcribe.Runner
	mu        sync.Mutex
	machine   state.Machine
	phone     *phone
	device    string
	pcm       []byte
	stopSent  bool
	copy      func(string) error
	indicator func(string)
}

type phone struct {
	conn    *websocket.Conn
	writeMu sync.Mutex
}

func New(c config.Config, log *slog.Logger) *Server {
	return &Server{config: c, log: log, runner: transcribe.Runner{Binary: c.WhisperBin, Model: c.WhisperModel, Threads: c.WhisperThreads}, machine: state.New(), copy: copyClipboard, indicator: newIndicator(c.IndicatorBin, log)}
}

func (s *Server) WebSocketHandler(w http.ResponseWriter, r *http.Request) {
	if s.config.Token != "" && r.URL.Query().Get("token") != s.config.Token {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	p := &phone{conn: conn}
	s.mu.Lock()
	previous := s.phone
	s.phone, s.device = p, "unknown"
	s.mu.Unlock()
	if previous != nil {
		previous.conn.Close()
	}
	s.log.Info("phone connected", "remote", r.RemoteAddr)
	defer s.disconnect(p)
	for {
		kind, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		switch kind {
		case websocket.BinaryMessage:
			s.appendPCM(p, data)
		case websocket.TextMessage:
			s.handleMessage(p, data)
		}
	}
}

func (s *Server) appendPCM(p *phone, data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.phone == p && (s.machine.Phase() == state.Recording || s.machine.Phase() == state.Transcribing && s.stopSent) {
		s.pcm = append(s.pcm, data...)
	}
}

func (s *Server) handleMessage(p *phone, data []byte) {
	message, err := protocol.Parse(data)
	if err != nil {
		s.log.Warn("invalid phone message", "error", err)
		return
	}
	s.mu.Lock()
	if s.phone != p {
		s.mu.Unlock()
		return
	}
	if message.Type == protocol.HelloType {
		s.device = message.Device
		s.mu.Unlock()
		s.log.Info("phone identified", "device", message.Device)
		return
	}
	if message.Type == protocol.StoppedType && s.machine.Phase() == state.Transcribing && s.stopSent {
		pcm := append([]byte(nil), s.pcm...)
		s.stopSent = false
		s.mu.Unlock()
		go s.transcribe(pcm)
		return
	}
	s.mu.Unlock()
}

func (s *Server) disconnect(p *phone) {
	s.mu.Lock()
	if s.phone == p {
		s.phone, s.device = nil, ""
	}
	// If the phone drops before it confirms the stop, no stopped message will
	// ever arrive; transcribe what we have so the session cannot stay stuck.
	waitingForStopped := s.machine.Phase() == state.Transcribing && s.stopSent
	s.mu.Unlock()
	p.conn.Close()
	s.log.Info("phone disconnected")
	if waitingForStopped {
		s.transcribeNowAfterStopFailure()
	}
}

func (s *Server) ControlHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok\n")) })
	mux.HandleFunc("GET /status", s.status)
	mux.HandleFunc("POST /start", s.start)
	mux.HandleFunc("POST /stop", s.stop)
	mux.HandleFunc("POST /toggle", s.toggle)
	return mux
}

func (s *Server) status(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	json.NewEncoder(w).Encode(map[string]any{"state": s.machine.Phase(), "phone_connected": s.phone != nil, "device": s.device, "audio_bytes": len(s.pcm)})
}

func (s *Server) toggle(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	phase := s.machine.Phase()
	s.mu.Unlock()
	if phase == state.Idle {
		s.start(w, nil)
		return
	}
	if phase == state.Recording {
		s.stop(w, nil)
		return
	}
	http.Error(w, "transcription already in progress", http.StatusConflict)
}

func (s *Server) start(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	if s.phone == nil {
		s.mu.Unlock()
		http.Error(w, "no phone connected", http.StatusServiceUnavailable)
		return
	}
	if err := s.machine.Start(); err != nil {
		s.mu.Unlock()
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	s.pcm, s.stopSent = nil, false
	p := s.phone
	s.mu.Unlock()
	if err := p.send(protocol.StartType); err != nil {
		s.failStart(err)
		http.Error(w, "send start: "+err.Error(), http.StatusBadGateway)
		return
	}
	s.log.Info("recording started")
	s.indicator("recording")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) failStart(err error) {
	s.mu.Lock()
	s.machine.Cancel()
	s.mu.Unlock()
	s.log.Error("start phone recording", "error", err)
}

func (s *Server) stop(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	if err := s.machine.Stop(); err != nil {
		s.mu.Unlock()
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	p := s.phone
	s.stopSent = true
	s.mu.Unlock()
	if p == nil || p.send(protocol.StopType) != nil {
		s.transcribeNowAfterStopFailure()
		w.WriteHeader(http.StatusAccepted)
		return
	}
	s.log.Info("recording stop requested")
	s.indicator("transcribing")
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) transcribeNowAfterStopFailure() {
	s.mu.Lock()
	pcm := append([]byte(nil), s.pcm...)
	s.stopSent = false
	s.mu.Unlock()
	go s.transcribe(pcm)
}

func (p *phone) send(kind string) error {
	data, err := protocol.Control(kind)
	if err != nil {
		return err
	}
	p.writeMu.Lock()
	defer p.writeMu.Unlock()
	return p.conn.WriteMessage(websocket.TextMessage, data)
}

func (s *Server) transcribe(pcm []byte) {
	defer func() { s.mu.Lock(); s.machine.Complete(); s.mu.Unlock() }()
	if len(pcm) == 0 {
		s.log.Error("transcription failed", "error", "no audio received")
		s.indicator("failed")
		return
	}
	if err := audio.WriteWAV(s.config.WAVPath, pcm); err != nil {
		s.log.Error("write WAV", "error", err)
		s.indicator("failed")
		return
	}
	s.log.Info("transcription started", "bytes", len(pcm), "audio_ms", audio.DurationMilliseconds(len(pcm)))
	text, duration, err := s.runner.Run(context.Background(), s.config.WAVPath)
	if err != nil {
		s.log.Error("transcription failed", "error", err, "duration", duration)
		s.indicator("failed")
		return
	}
	s.log.Info("transcription complete", "duration", duration, "text", text)
	if err := s.copy(text); err != nil {
		s.log.Error("copy clipboard", "error", err)
		return
	}
	s.log.Info("copied transcription to clipboard")
	s.indicator("copied")
}

func newIndicator(binary string, log *slog.Logger) func(string) {
	if binary == "" {
		return func(string) {}
	}
	return func(state string) {
		if err := exec.Command(binary, "set", state).Run(); err != nil {
			log.Debug("update indicator", "error", err)
		}
	}
}

func (s *Server) ListenAndServe() error {
	ws := &http.Server{Addr: s.config.WSAddr, Handler: http.HandlerFunc(s.WebSocketHandler), ReadHeaderTimeout: 5 * time.Second}
	control := &http.Server{Addr: s.config.ControlAddr, Handler: s.ControlHandler(), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		s.log.Info("websocket listening", "address", s.config.WSAddr)
		if err := ws.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.log.Error("websocket server", "error", err)
		}
	}()
	s.log.Info("control listening", "address", s.config.ControlAddr)
	return control.ListenAndServe()
}
