package utils

import (
	"crypto/sha256"
	"encoding/hex"
)

// ShortHash returns the first 12 hex characters of the SHA-256 of s.
// Used to give flaky occurrences a stable identity across runs so reports can
// be diffed and de-duplicated.
func ShortHash(parts ...string) string {
	h := sha256.New()
	for _, part := range parts {
		_, _ = h.Write([]byte(part))
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))[:12]
}
