package overlay

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestHandlerAppliesStateAndRejectsUnknown(t *testing.T) {
	store := NewStore()
	srv := httptest.NewServer(Handler(store))
	defer srv.Close()

	resp, err := http.PostForm(srv.URL+"/set", url.Values{"state": {"recording"}})
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 204 {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}
	state, age := store.Current()
	if state != "recording" || age > time.Second {
		t.Fatalf("state = %q age %v, want fresh recording", state, age)
	}

	resp, err = http.PostForm(srv.URL+"/set", url.Values{"state": {"bogus"}})
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("unknown state status = %d, want 400", resp.StatusCode)
	}
}

func TestCurrentExpiresTimedStates(t *testing.T) {
	store := NewStore()
	store.Set("copied")
	if state, _ := store.Current(); state != "copied" {
		t.Fatalf("state = %q, want copied", state)
	}
	store.mu.Lock()
	store.shownAt = time.Now().Add(-2 * time.Second)
	store.mu.Unlock()
	if state, _ := store.Current(); state != "" {
		t.Fatalf("state = %q after hide window, want empty", state)
	}
}
