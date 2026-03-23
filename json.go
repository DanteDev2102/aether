package aether

import (
	"encoding/json"
	"io"
)

// JSONEngine defines the interface for JSON encoding and decoding.
type JSONEngine interface {
	Decode(r io.Reader, v any) error
	Encode(w io.Writer, v any) error
}

type stdJSONEngine struct{}

// Decode reads JSON data from a reader into the provided value.
func (stdJSONEngine) Decode(r io.Reader, v any) error {
	return json.NewDecoder(r).Decode(v)
}

// Encode writes a value as JSON to the provided writer.
func (stdJSONEngine) Encode(w io.Writer, v any) error {
	return json.NewEncoder(w).Encode(v)
}
