package server

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"localwhisper/receiver/internal/config"
	"localwhisper/receiver/internal/state"
)

func TestHealthAndStatusReportInitialReceiverState(t *testing.T) {
	s := New(config.Config{WhisperThreads: 8}, slog.Default())
	handler := s.ControlHandler()
	for _, path := range []string{"/health", "/status"} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s status = %d", path, recorder.Code)
		}
	}
}

func TestStartRequiresConnectedPhone(t *testing.T) {
	s := New(config.Config{WhisperThreads: 8}, slog.Default())
	recorder := httptest.NewRecorder()
	s.ControlHandler().ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/start", nil))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestDisconnectWhileAwaitingStoppedFlushesBufferedAudio(t *testing.T) {
	s := New(config.Config{WhisperThreads: 8}, slog.Default())
	ws := httptest.NewServer(http.HandlerFunc(s.WebSocketHandler))
	defer ws.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(ws.URL, "http"), nil)
	if err != nil {
		t.Fatalf("dial phone websocket: %v", err)
	}
	defer conn.Close()

	p := &phone{conn: conn}
	s.mu.Lock()
	s.phone = p
	if err := s.machine.Start(); err != nil {
		t.Fatalf("start machine: %v", err)
	}
	if err := s.machine.Stop(); err != nil {
		t.Fatalf("stop machine: %v", err)
	}
	s.pcm = []byte("fake")
	s.stopSent = true
	s.mu.Unlock()

	s.disconnect(p)

	deadline := time.Now().Add(2 * time.Second)
	for {
		s.mu.Lock()
		phase := s.machine.Phase()
		s.mu.Unlock()
		if phase == state.Idle {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("machine stuck in %s after phone disconnect", phase)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
