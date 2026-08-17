package state

import "testing"

func TestMachineFollowsDictationLifecycle(t *testing.T) {
	m := New()
	if err := m.Start(); err != nil {
		t.Fatal(err)
	}
	if err := m.Stop(); err != nil {
		t.Fatal(err)
	}
	if err := m.Complete(); err != nil {
		t.Fatal(err)
	}
	if m.Phase() != Idle {
		t.Fatalf("phase = %s", m.Phase())
	}
}

func TestMachineRejectsInvalidTransition(t *testing.T) {
	m := New()
	if err := m.Stop(); err == nil {
		t.Fatal("Stop() succeeded from idle")
	}
}
