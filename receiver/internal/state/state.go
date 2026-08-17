package state

import "fmt"

type Phase string

const (
	Idle         Phase = "IDLE"
	Recording    Phase = "RECORDING"
	Transcribing Phase = "TRANSCRIBING"
)

type Machine struct{ phase Phase }

func New() Machine              { return Machine{phase: Idle} }
func (m *Machine) Phase() Phase { return m.phase }
func (m *Machine) Start() error {
	if m.phase != Idle {
		return fmt.Errorf("cannot start while %s", m.phase)
	}
	m.phase = Recording
	return nil
}
func (m *Machine) Stop() error {
	if m.phase != Recording {
		return fmt.Errorf("cannot stop while %s", m.phase)
	}
	m.phase = Transcribing
	return nil
}
func (m *Machine) Complete() error {
	if m.phase != Transcribing {
		return fmt.Errorf("cannot complete while %s", m.phase)
	}
	m.phase = Idle
	return nil
}

func (m *Machine) Cancel() error {
	if m.phase != Recording {
		return fmt.Errorf("cannot cancel while %s", m.phase)
	}
	m.phase = Idle
	return nil
}
