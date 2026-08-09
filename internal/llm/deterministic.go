package llm

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/mdryaaan/flakehunter/internal/verdict"
)

// BaselineDisclaimer is attached to every verdict this provider produces and is
// printed by the eval and report commands.
//
// It exists because a number produced by this provider must never be mistaken
// for a measurement of a language model. Reporting rule-based accuracy as if it
// were LLM accuracy would misrepresent the tool's headline metric.
const BaselineDisclaimer = "rule-based baseline (no language model involved)"

// Deterministic is a transparent keyword-and-pattern classifier.
//
// It is NOT a language model and is not a substitute for one. It exists for two
// legitimate reasons:
//
//  1. Tests and CI need a provider that is offline, instant and reproducible.
//  2. Evaluation needs a control. "The model scored 82%" is meaningless on its
//     own; "the model scored 82% where grep scores 71%" is a result. A classifier
//     that cannot beat its own baseline is not earning its inference cost.
//
// Its verdicts are always marked with BaselineDisclaimer so downstream output
// cannot present them as model output.
type Deterministic struct {
	rules []rule
}

type rule struct {
	category verdict.Category
	weight   float64
	pattern  *regexp.Regexp
}

// NewDeterministic builds the baseline classifier.
func NewDeterministic() *Deterministic {
	specs := []struct {
		category verdict.Category
		weight   float64
		expr     string
	}{
		// network_timeout
		{verdict.NetworkTimeout, 3.0, `(?i)\bi/o timeout\b`},
		{verdict.NetworkTimeout, 3.0, `(?i)\bdial tcp\b`},
		{verdict.NetworkTimeout, 2.5, `(?i)connection (refused|reset by peer)`},
		{verdict.NetworkTimeout, 2.5, `(?i)tls handshake timeout`},
		{verdict.NetworkTimeout, 2.0, `(?i)context deadline exceeded`},
		{verdict.NetworkTimeout, 2.0, `(?i)(no such host|temporary failure in name resolution)`},
		{verdict.NetworkTimeout, 1.5, `(?i)could not resolve host`},

		// race_condition
		{verdict.RaceCondition, 4.0, `(?i)WARNING: DATA RACE`},
		{verdict.RaceCondition, 3.0, `(?i)send on closed channel`},
		{verdict.RaceCondition, 3.0, `(?i)concurrent map (read and map )?write`},
		{verdict.RaceCondition, 2.5, `(?i)all goroutines are asleep - deadlock`},
		{verdict.RaceCondition, 2.0, `(?i)race detector`},
		{verdict.RaceCondition, 1.5, `(?i)(previous write|previous read) at 0x`},

		// infra_flake
		{verdict.InfraFlake, 3.5, `(?i)runner has received a shutdown signal`},
		{verdict.InfraFlake, 3.5, `(?i)lost communication with the server`},
		{verdict.InfraFlake, 3.0, `(?i)the runner has received a shutdown`},
		{verdict.InfraFlake, 2.5, `(?i)failed to download action`},
		{verdict.InfraFlake, 2.5, `(?i)the operation was canceled.*runner`},
		{verdict.InfraFlake, 2.0, `(?i)docker daemon.*not running`},
		{verdict.InfraFlake, 2.0, `(?i)spot instance.*(reclaim|interrupt)`},

		// resource_exhaustion
		{verdict.ResourceExhaustion, 4.0, `(?i)no space left on device`},
		{verdict.ResourceExhaustion, 4.0, `(?i)\boom-?killed\b`},
		{verdict.ResourceExhaustion, 3.5, `(?i)out of memory`},
		{verdict.ResourceExhaustion, 3.0, `(?i)cannot allocate memory`},
		{verdict.ResourceExhaustion, 3.0, `(?i)too many open files`},
		{verdict.ResourceExhaustion, 2.5, `(?i)signal: killed`},
		{verdict.ResourceExhaustion, 2.0, `(?i)exit code 137`},

		// test_order_dependency
		{verdict.TestOrderDependency, 3.5, `(?i)-shuffle`},
		{verdict.TestOrderDependency, 3.0, `(?i)passes in isolation`},
		{verdict.TestOrderDependency, 3.0, `(?i)shared fixture`},
		{verdict.TestOrderDependency, 2.5, `(?i)leaked state`},
		{verdict.TestOrderDependency, 2.5, `(?i)(already exists|duplicate).*(registry|registered)`},
		{verdict.TestOrderDependency, 2.0, `(?i)test order`},

		// genuine_bug
		{verdict.GenuineBug, 3.0, `(?i)nil pointer dereference`},
		{verdict.GenuineBug, 3.0, `(?i)index out of range`},
		{verdict.GenuineBug, 2.5, `(?i)expected .* (got|but got) `},
		{verdict.GenuineBug, 2.0, `(?i)assertion failed`},
		{verdict.GenuineBug, 2.0, `(?i)invalid memory address`},
	}

	rules := make([]rule, 0, len(specs))
	for _, spec := range specs {
		rules = append(rules, rule{
			category: spec.category,
			weight:   spec.weight,
			pattern:  regexp.MustCompile(spec.expr),
		})
	}

	return &Deterministic{rules: rules}
}

// Name identifies the provider.
func (d *Deterministic) Name() string { return ProviderDeterministic }

// Model identifies the "model" — there isn't one, and the string says so.
func (d *Deterministic) Model() string { return "rule-based-v1" }

// Classify scores the excerpt against every rule and returns the winning
// category, with confidence derived from the margin over the runner-up.
func (d *Deterministic) Classify(_ context.Context, req Request) (verdict.Verdict, error) {
	scores := make(map[verdict.Category]float64)
	matched := make(map[verdict.Category][]string)

	for _, r := range d.rules {
		if loc := r.pattern.FindStringIndex(req.Excerpt); loc != nil {
			scores[r.category] += r.weight
			matched[r.category] = append(matched[r.category], lineAt(req.Excerpt, loc[0]))
		}
	}

	if len(scores) == 0 {
		return verdict.Verdict{
			Category:            verdict.Unknown,
			Confidence:          0.2,
			Explanation:         "No known failure pattern matched this excerpt. " + BaselineDisclaimer + ".",
			SuggestedMitigation: verdict.MitigationFor(verdict.Unknown).Summary,
		}, nil
	}

	type scored struct {
		category verdict.Category
		score    float64
	}
	ranked := make([]scored, 0, len(scores))
	for category, score := range scores {
		ranked = append(ranked, scored{category, score})
	}
	// Deterministic ordering: score first, then name, so map iteration order
	// can never change the answer.
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		return ranked[i].category < ranked[j].category
	})

	best := ranked[0]
	runnerUp := 0.0
	if len(ranked) > 1 {
		runnerUp = ranked[1].score
	}

	// Confidence rises with both absolute score and the margin over second
	// place, and is capped below 1 — a keyword match is never certainty.
	margin := best.score - runnerUp
	confidence := 0.45 + 0.10*best.score + 0.08*margin
	if confidence > 0.92 {
		confidence = 0.92
	}

	// Several rules routinely match the same line; quoting it three times is
	// noise, not corroboration.
	citations := dedupe(matched[best.category])
	if len(citations) > 3 {
		citations = citations[:3]
	}

	return verdict.Verdict{
		Category:   best.category,
		Confidence: confidence,
		Explanation: fmt.Sprintf("Matched %s for %s. %s.",
			plural(len(matched[best.category])), best.category.Label(), BaselineDisclaimer),
		CitedLines:          citations,
		SuggestedMitigation: verdict.MitigationFor(best.category).Summary,
	}, nil
}

// dedupe removes repeated citations while preserving first-seen order.
func dedupe(lines []string) []string {
	seen := make(map[string]struct{}, len(lines))
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if _, ok := seen[line]; ok {
			continue
		}
		seen[line] = struct{}{}
		out = append(out, line)
	}
	return out
}

func plural(n int) string {
	if n == 1 {
		return "1 pattern"
	}
	return fmt.Sprintf("%d patterns", n)
}

// lineAt returns the whole line containing byte offset idx, so citations are
// real lines from the excerpt rather than fragments.
func lineAt(s string, idx int) string {
	start := strings.LastIndexByte(s[:idx], '\n') + 1
	end := strings.IndexByte(s[idx:], '\n')
	if end < 0 {
		return strings.TrimSpace(s[start:])
	}
	return strings.TrimSpace(s[start : idx+end])
}
