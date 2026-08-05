package report

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/mdryaaan/flakehunter/internal/verdict"
)

// WriteMarkdown renders a full classification report.
func WriteMarkdown(w io.Writer, res ClassifiedResult) error {
	var b strings.Builder

	b.WriteString("# Flake report\n\n")
	fmt.Fprintf(&b, "**Repository:** `%s`  \n", res.Repo)
	fmt.Fprintf(&b, "**Source:** %s  \n", res.Source)
	fmt.Fprintf(&b, "**Classifier:** %s  \n", res.ProviderLabel())
	fmt.Fprintf(&b, "**Generated:** %s\n\n", res.GeneratedAt.UTC().Format("2006-01-02 15:04 UTC"))

	if res.Baseline {
		b.WriteString("> [!NOTE]\n> These verdicts come from the deterministic rule-based " +
			"baseline, not a language model. They are reproducible and free, and exist to " +
			"establish the floor an LLM must beat.\n\n")
	}

	writeSummaryTable(&b, res)
	writeDetail(&b, res)

	if _, err := io.WriteString(w, b.String()); err != nil {
		return fmt.Errorf("writing markdown report: %w", err)
	}
	return nil
}

func writeSummaryTable(b *strings.Builder, res ClassifiedResult) {
	counts := res.CountsByCategory()

	total := 0
	for _, n := range counts {
		total += n
	}

	b.WriteString("## Summary\n\n")
	if total == 0 {
		b.WriteString("No flaky occurrences were classified.\n\n")
		return
	}

	fmt.Fprintf(b, "%d flaky occurrence(s) classified.\n\n", total)
	b.WriteString("| Category | Count | Share | Typical owner |\n")
	b.WriteString("| --- | ---: | ---: | --- |\n")

	for _, category := range verdict.AllCategories() {
		n := counts[category]
		if n == 0 {
			continue
		}
		share := float64(n) / float64(total) * 100
		fmt.Fprintf(b, "| %s | %d | %.0f%% | %s |\n",
			category.Label(), n, share, verdict.MitigationFor(category).Owner)
	}
	b.WriteString("\n")
}

func writeDetail(b *strings.Builder, res ClassifiedResult) {
	if len(res.Items) == 0 {
		return
	}

	items := make([]Classified, len(res.Items))
	copy(items, res.Items)

	// Most actionable first: a genuine bug hiding behind a rerun outranks a
	// runner blip, and within a severity band the confident calls come first.
	sort.SliceStable(items, func(i, j int) bool {
		si, sj := items[i].Verdict.Category.Severity(), items[j].Verdict.Category.Severity()
		if si != sj {
			return si > sj
		}
		return items[i].Verdict.Confidence > items[j].Verdict.Confidence
	})

	b.WriteString("## Occurrences\n\n")

	for _, item := range items {
		occ := item.Occurrence
		fmt.Fprintf(b, "### `%s` — %s\n\n", occ.JobName, item.Verdict.Category.Label())

		if item.Error != "" {
			fmt.Fprintf(b, "> Classification failed: %s\n\n", item.Error)
			continue
		}

		fmt.Fprintf(b, "- **Commit:** `%s` on `%s`\n", shortSHA(occ.HeadSHA), occ.Branch)
		fmt.Fprintf(b, "- **Workflow:** %s (`%s`)\n", occ.WorkflowName, occ.WorkflowFile)
		fmt.Fprintf(b, "- **Failure rate:** %.0f%% (%d attempts)\n",
			occ.FailureRate*100, occ.TotalAttempts)
		fmt.Fprintf(b, "- **Confidence:** %.2f\n", item.Verdict.Confidence)
		if item.Verdict.Downgraded {
			fmt.Fprintf(b, "- **Note:** demoted to unknown from `%s` for low confidence\n",
				item.Verdict.RawCategory)
		}
		b.WriteString("\n")

		fmt.Fprintf(b, "%s\n\n", item.Verdict.Explanation)

		if len(item.Verdict.CitedLines) > 0 {
			b.WriteString("**Evidence**\n\n```\n")
			for _, line := range item.Verdict.CitedLines {
				b.WriteString(strings.TrimRight(line, "\n") + "\n")
			}
			b.WriteString("```\n\n")
		}
		if len(item.Verdict.Hallucinated) > 0 {
			fmt.Fprintf(b, "> %d cited line(s) were dropped because they do not appear in the log excerpt.\n\n",
				len(item.Verdict.Hallucinated))
		}

		m := verdict.MitigationFor(item.Verdict.Category)
		fmt.Fprintf(b, "**Mitigation** — %s _(owner: %s)_\n\n", m.Summary, m.Owner)
		for _, step := range m.Steps {
			fmt.Fprintf(b, "- %s\n", step)
		}
		b.WriteString("\n")

		if len(occ.FailedRuns) > 0 {
			fmt.Fprintf(b, "[Failing run](%s)", occ.FailedRuns[len(occ.FailedRuns)-1].URL)
			if len(occ.PassedRuns) > 0 {
				fmt.Fprintf(b, " · [Passing rerun](%s)", occ.PassedRuns[len(occ.PassedRuns)-1].URL)
			}
			b.WriteString("\n\n")
		}
		b.WriteString("---\n\n")
	}
}

func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}
