package report

import (
	"encoding/json"
	"fmt"
	"io"
)

// WriteJSON serialises any result type as indented JSON.
func WriteJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("encoding json report: %w", err)
	}
	return nil
}

// ReadJSON deserialises a previously written report.
func ReadJSON(r io.Reader, v any) error {
	if err := json.NewDecoder(r).Decode(v); err != nil {
		return fmt.Errorf("decoding json report: %w", err)
	}
	return nil
}
