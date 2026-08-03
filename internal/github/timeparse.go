package github

import (
	"fmt"
	"time"
)

// parseFixtureTime accepts RFC3339, with or without a zone, so hand-written
// fixtures do not have to be pedantic about format.
func parseFixtureTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognised timestamp %q", s)
}
