package aether

import (
	"encoding/xml"
	"io"
)

// XMLEngine defines the interface for XML encoding and decoding.
type XMLEngine interface {
	Decode(r io.Reader, v any) error
	Encode(w io.Writer, v any) error
}

type stdXMLEngine struct{}

// Decode reads XML data from a reader into the provided value.
func (stdXMLEngine) Decode(r io.Reader, v any) error {
	return xml.NewDecoder(r).Decode(v)
}

// Encode writes a value as XML to the provided writer.
func (stdXMLEngine) Encode(w io.Writer, v any) error {
	return xml.NewEncoder(w).Encode(v)
}
