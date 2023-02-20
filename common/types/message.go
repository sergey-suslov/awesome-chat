package types

import (
	"bytes"
	"encoding/gob"
)

const (
	MessageTypeConnect uint8 = 1 + iota
	MessageTypeNewRoom
	MessageTypeConnected
	MessageTypeConnectionError
)

type Message struct {
	MessageType byte
	Data        []byte
}

type ConnectWithNameMessage struct {
	Name string
}

func EncodeMessage[T any](t T) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(t)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func DecodeMessage[T any](t *T, data []byte) error {
	buf := bytes.NewBuffer(data)
	dec := gob.NewDecoder(buf)
	err := dec.Decode(&t)
	if err != nil {
		return err
	}
	return nil
}

func ComposeMessage(messageType uint8, encoded []byte) []byte {
	msg := []byte{messageType}
	return append(msg, encoded...)
}
