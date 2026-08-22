// Package overlay renders the desktop status pill and exposes the
// `set <state>` contract shared with the Linux GTK indicator.
package overlay

import (
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// Meta describes how a state renders and when it hides itself.
type Meta struct {
	Text      string
	HideAfter time.Duration // 0 keeps the state until replaced
}

var states = map[string]Meta{
	"recording":    {},
	"transcribing": {},
	"copied":       {HideAfter: 1800 * time.Millisecond},
	"failed":       {Text: "Transcription failed", HideAfter: 3500 * time.Millisecond},
	"disconnected": {Text: "Mobile not connected", HideAfter: 3500 * time.Millisecond},
}

// Label returns the text caption shown next to a state's glyph.
func Label(name string) string { return states[name].Text }

// Valid reports whether name is a renderable overlay state.
func Valid(name string) bool {
	_, ok := states[name]
	return ok
}

// Store holds the currently displayed state for a running overlay.
type Store struct {
	mu      sync.Mutex
	state   string
	shownAt time.Time
}

func NewStore() *Store { return &Store{} }

func (s *Store) Set(state string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !Valid(state) {
		return
	}
	s.state, s.shownAt = state, time.Now()
}

// Current returns the active state and its age, or "" when nothing is shown.
// Timed states expire on read so the window can render fully transparent.
func (s *Store) Current() (string, time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == "" {
		return "", 0
	}
	age := time.Since(s.shownAt)
	if hide := states[s.state].HideAfter; hide > 0 && age > hide {
		return "", 0
	}
	return s.state, age
}

// Handler serves POST /set for a running overlay process.
func Handler(store *Store) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /set", func(w http.ResponseWriter, r *http.Request) {
		state := r.FormValue("state")
		if !Valid(state) {
			http.Error(w, "unknown overlay state", http.StatusBadRequest)
			return
		}
		store.Set(state)
		w.WriteHeader(http.StatusNoContent)
	})
	return mux
}

// Listen starts the overlay control server; intended to run in a goroutine.
func Listen(addr string, store *Store) error {
	return http.ListenAndServe(addr, Handler(store))
}

// Post sends a state change to a running overlay's control server.
func Post(addr, state string) error {
	resp, err := http.PostForm("http://"+addr+"/set", url.Values{"state": {state}})
	if err != nil {
		return fmt.Errorf("overlay set %s: %w", state, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("overlay set %s: %s", state, resp.Status)
	}
	return nil
}
