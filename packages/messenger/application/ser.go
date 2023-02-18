package application

import (
	"bytes"
	"encoding/gob"
)

func DecodeBody[T any](t *T, data []byte) error {
	var buf bytes.Buffer
	dec := gob.NewDecoder(&buf)
	err := dec.Decode(&t)
	if err != nil {
		return err
	}
	return nil
}
