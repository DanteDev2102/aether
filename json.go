package main

import (
	"encoding/json"
	"io"
)

type JSONEngine interface {
	Decode(r io.Reader, v any) error
	Encode(w io.Writer, v any) error
}

type stdJSONEngine struct{}

func (stdJSONEngine) Decode(r io.Reader, v any) error {
	return json.NewDecoder(r).Decode(v)
}

func (stdJSONEngine) Encode(w io.Writer, v any) error {
	return json.NewEncoder(w).Encode(v)
}
