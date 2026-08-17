package server

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"localwhisper/receiver/internal/config"
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
