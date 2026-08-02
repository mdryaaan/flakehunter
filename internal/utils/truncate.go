// Package utils holds small helpers shared across flakehunter's packages.
package utils

import "strings"

// TruncateRunes shortens s to at most max runes, appending an ellipsis when it
// had to cut. Rune-aware so multi-byte characters are never split in half.
func TruncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	if max <= 3 {
		return string(runes[:max])
	}
	return string(runes[:max-3]) + "..."
}

// LastLines returns the final n lines of s, preserving their order.
func LastLines(s string, n int) []string {
	if n <= 0 {
		return nil
	}
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) <= n {
		return lines
	}
	return lines[len(lines)-n:]
}

// CollapseBlankLines replaces runs of blank lines with a single blank line.
// CI logs are full of padding that wastes scarce LLM context.
func CollapseBlankLines(s string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	blank := false

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			if blank {
				continue
			}
			blank = true
		} else {
			blank = false
		}
		out = append(out, line)
	}

	return strings.Join(out, "\n")
}
