package jsonutil

import (
	"bytes"
	"encoding/json"
	"io"
)

// MarshalNoEscape serializes v to JSON without HTML escaping of &, <, >.
// Returns the JSON bytes with no trailing newline.
func MarshalNoEscape(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// NewEncoder returns a json.Encoder configured to write to w without HTML escaping.
// Use this for streaming JSON output to io.Writer destinations.
func NewEncoder(w io.Writer) *json.Encoder {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc
}
