package llm

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/mdryaaan/flakehunter/internal/verdict"
)

// ErrMalformed marks a response that could not be parsed into the schema.
var ErrMalformed = errors.New("malformed model response")

// rawVerdict mirrors the JSON contract exactly. It is a separate type from
// verdict.Verdict so a model cannot set internal fields such as Downgraded.
type rawVerdict struct {
	Category            string   `json:"category"`
	Confidence          float64  `json:"confidence"`
	Explanation         string   `json:"explanation"`
	CitedLines          []string `json:"cited_lines"`
	SuggestedMitigation string   `json:"suggested_mitigation"`
}

// SchemaJSON is the contract shown to the model in the prompt.
const SchemaJSON = `{
  "category": "one of: network_timeout | race_condition | infra_flake | resource_exhaustion | test_order_dependency | genuine_bug | unknown",
  "confidence": 0.0,
  "explanation": "one or two sentences, plain English",
  "cited_lines": ["exact lines copied verbatim from the excerpt"],
  "suggested_mitigation": "one concrete action"
}`

// ParseVerdict extracts and validates a verdict from a raw model response.
//
// Models wrap JSON in prose or fenced code blocks often enough that demanding
// a bare object would fail constantly, so the object is located inside the
// response rather than assumed to be the whole of it. Anything that still does
// not satisfy the schema is rejected — the caller retries once and then gives
// up honestly rather than guessing.
func ParseVerdict(raw string) (verdict.Verdict, error) {
	payload, err := extractJSONObject(raw)
	if err != nil {
		return verdict.Verdict{}, err
	}

	var rv rawVerdict
	decoder := json.NewDecoder(strings.NewReader(payload))
	if err := decoder.Decode(&rv); err != nil {
		return verdict.Verdict{}, fmt.Errorf("%w: %v", ErrMalformed, err)
	}

	category, err := verdict.ParseCategory(strings.ToLower(strings.TrimSpace(rv.Category)))
	if err != nil {
		return verdict.Verdict{}, fmt.Errorf("%w: %v", ErrMalformed, err)
	}

	v := verdict.Verdict{
		Category:            category,
		Confidence:          rv.Confidence,
		Explanation:         strings.TrimSpace(rv.Explanation),
		CitedLines:          rv.CitedLines,
		SuggestedMitigation: strings.TrimSpace(rv.SuggestedMitigation),
	}

	if err := v.Validate(); err != nil {
		return verdict.Verdict{}, fmt.Errorf("%w: %v", ErrMalformed, err)
	}

	return v, nil
}

// extractJSONObject finds the first balanced top-level JSON object in s,
// tolerating markdown fences and surrounding commentary.
func extractJSONObject(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("%w: empty response", ErrMalformed)
	}

	// Strip a fenced block if present.
	if idx := strings.Index(s, "```"); idx >= 0 {
		rest := s[idx+3:]
		if nl := strings.IndexByte(rest, '\n'); nl >= 0 {
			rest = rest[nl+1:]
		}
		if end := strings.Index(rest, "```"); end >= 0 {
			rest = rest[:end]
		}
		s = strings.TrimSpace(rest)
	}

	start := strings.IndexByte(s, '{')
	if start < 0 {
		return "", fmt.Errorf("%w: no JSON object found", ErrMalformed)
	}

	depth := 0
	inString := false
	escaped := false

	for i := start; i < len(s); i++ {
		c := s[i]
		switch {
		case escaped:
			escaped = false
		case c == '\\' && inString:
			escaped = true
		case c == '"':
			inString = !inString
		case inString:
			// ignore structural characters inside strings
		case c == '{':
			depth++
		case c == '}':
			depth--
			if depth == 0 {
				return s[start : i+1], nil
			}
		}
	}

	return "", fmt.Errorf("%w: unbalanced JSON object", ErrMalformed)
}
