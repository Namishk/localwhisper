package protocol

import "testing"

func TestParseHello(t *testing.T) {
	m, err := Parse([]byte(`{"type":"hello","device":"Pixel"}`))
	if err != nil {
		t.Fatal(err)
	}
	if m.Device != "Pixel" {
		t.Fatalf("device = %q", m.Device)
	}
}

func TestParseRejectsUnknownType(t *testing.T) {
	if _, err := Parse([]byte(`{"type":"audio"}`)); err == nil {
		t.Fatal("Parse() succeeded")
	}
}
