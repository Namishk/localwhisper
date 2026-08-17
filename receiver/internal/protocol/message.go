package protocol

import (
	"encoding/json"
	"fmt"
)

const (
	HelloType   = "hello"
	StoppedType = "stopped"
	StartType   = "start"
	StopType    = "stop"
)

type Message struct {
	Type   string `json:"type"`
	Device string `json:"device,omitempty"`
}

func Parse(data []byte) (Message, error) {
	var message Message
	if err := json.Unmarshal(data, &message); err != nil {
		return Message{}, fmt.Errorf("decode message: %w", err)
	}
	if message.Type != HelloType && message.Type != StoppedType {
		return Message{}, fmt.Errorf("unsupported message type %q", message.Type)
	}
	return message, nil
}

func Control(kind string) ([]byte, error) { return json.Marshal(Message{Type: kind}) }
