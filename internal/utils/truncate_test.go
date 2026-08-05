package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTruncateRunes(t *testing.T) {
	tests := []struct {
		in   string
		max  int
		want string
	}{
		{"hello", 10, "hello"},
		{"hello world", 8, "hello..."},
		{"hello", 0, ""},
		{"hello", 2, "he"},
		{"héllo wörld", 8, "héllo..."},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			assert.Equal(t, tt.want, TruncateRunes(tt.in, tt.max))
		})
	}
}

func TestLastLines(t *testing.T) {
	body := "one\ntwo\nthree\nfour\n"
	assert.Equal(t, []string{"three", "four"}, LastLines(body, 2))
	assert.Equal(t, []string{"one", "two", "three", "four"}, LastLines(body, 99))
	assert.Nil(t, LastLines(body, 0))
}

func TestCollapseBlankLines(t *testing.T) {
	in := "a\n\n\n\nb\n\nc"
	assert.Equal(t, "a\n\nb\n\nc", CollapseBlankLines(in))
}

func TestShortHash(t *testing.T) {
	a := ShortHash("sha", "ci.yml", "unit")
	b := ShortHash("sha", "ci.yml", "unit")
	c := ShortHash("sha", "ci.yml", "lint")

	assert.Equal(t, a, b)
	assert.NotEqual(t, a, c)
	assert.Len(t, a, 12)

	// Field separation must matter, or "ab"+"c" would collide with "a"+"bc".
	assert.NotEqual(t, ShortHash("ab", "c"), ShortHash("a", "bc"))
}
