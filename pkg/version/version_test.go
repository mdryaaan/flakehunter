package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCurrentAlwaysPopulated(t *testing.T) {
	got := Current()

	assert.NotEmpty(t, got.Version)
	assert.NotEmpty(t, got.GoVersion)
	assert.NotEmpty(t, got.Platform)
	assert.Contains(t, got.String(), "flakehunter")
}

func TestShortCommit(t *testing.T) {
	tests := []struct{ in, want string }{
		{"6a03c7012345abcdef", "6a03c70"},
		{"abc", "abc"},
		{"", ""},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, shortCommit(tt.in))
	}
}
