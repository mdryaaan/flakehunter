package verdict

import (
	"errors"
	"fmt"
	"strings"
)

// DefaultConfidenceFloor is the point below which a specific category is
// downgraded to Unknown. A confidently wrong label is worse than an honest
// "I don't know" — a maintainer can act on the second and is misled by the first.
const DefaultConfidenceFloor = 0.5

// Verdict is the structured result of classifying one flaky occurrence.
type Verdict struct {
	Category            Category `json:"category"`
	Confidence          float64  `json:"confidence"`
	Explanation         string   `json:"explanation"`
	CitedLines          []string `json:"cited_lines"`
	SuggestedMitigation string   `json:"suggested_mitigation"`

	// Set when the raw category was demoted for falling under the floor.
	Downgraded  bool     `json:"downgraded,omitempty"`
	RawCategory Category `json:"raw_category,omitempty"`
	// Citations the model produced that are not present in the excerpt.
	Hallucinated []string `json:"hallucinated_citations,omitempty"`
}

// ErrInvalid marks a verdict that failed schema validation.
var ErrInvalid = errors.New("invalid verdict")

// Validate checks the verdict against the schema contract: a known category, a
// confidence in [0,1] and a non-empty explanation.
func (v Verdict) Validate() error {
	if !v.Category.Valid() {
		return fmt.Errorf("%w: unknown category %q", ErrInvalid, v.Category)
	}
	if v.Confidence < 0 || v.Confidence > 1 {
		return fmt.Errorf("%w: confidence %.3f outside [0,1]", ErrInvalid, v.Confidence)
	}
	if strings.TrimSpace(v.Explanation) == "" {
		return fmt.Errorf("%w: empty explanation", ErrInvalid)
	}
	return nil
}

// ApplyConfidenceFloor demotes a low-confidence verdict to Unknown, preserving
// the original label so a reviewer can still see what the model leaned towards.
func (v Verdict) ApplyConfidenceFloor(floor float64) Verdict {
	if v.Category == Unknown || v.Confidence >= floor {
		return v
	}
	v.RawCategory = v.Category
	v.Category = Unknown
	v.Downgraded = true
	return v
}

// VerifyCitations drops any cited line that does not literally appear in the
// excerpt, recording it under Hallucinated.
//
// This is the tool's main defence against a fabricated justification: a model
// that invents a plausible-sounding log line would otherwise produce a verdict
// that reads as evidence-backed but is not. Comparison is on trimmed substrings
// because models routinely re-indent or clip what they quote.
func (v Verdict) VerifyCitations(excerpt string) Verdict {
	if len(v.CitedLines) == 0 {
		return v
	}

	haystack := excerpt
	kept := make([]string, 0, len(v.CitedLines))
	var bad []string

	for _, cited := range v.CitedLines {
		needle := strings.TrimSpace(cited)
		if needle == "" {
			continue
		}
		if strings.Contains(haystack, needle) {
			kept = append(kept, cited)
			continue
		}
		bad = append(bad, cited)
	}

	v.CitedLines = kept
	v.Hallucinated = bad
	return v
}

// Mitigation returns the curated advice for this verdict's category.
func (v Verdict) Mitigation() Mitigation { return MitigationFor(v.Category) }
