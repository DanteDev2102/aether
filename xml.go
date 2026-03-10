package aether

import (
	"encoding/xml"
	"io"
)

type XMLEngine interface {
	Decode(r io.Reader, v any) error
	Encode(w io.Writer, v any) error
}

type stdXMLEngine struct{}

func (stdXMLEngine) Decode(r io.Reader, v any) error {
	return xml.NewDecoder(r).Decode(v)
}

func (stdXMLEngine) Encode(w io.Writer, v any) error {
	return xml.NewEncoder(w).Encode(v)
}
