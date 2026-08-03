package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/mdryaaan/flakehunter/internal/extractor"
	"github.com/mdryaaan/flakehunter/internal/llm"
	"github.com/mdryaaan/flakehunter/internal/report"
)

var (
	classifyInput  string
	classifyOutput string
)

var classifyCmd = &cobra.Command{
	Use:   "classify",
	Short: "Classify the root cause of each flaky occurrence",
	Long: `classify downloads the failure log for every occurrence found by scan, reduces it
to a relevant excerpt, and asks the configured provider for a structured verdict.

Verdicts are validated against a fixed JSON schema, cited lines are checked
against the excerpt they claim to come from, and anything below the confidence
floor is reported as unknown rather than as a low-confidence guess.`,
	Example: `  flakehunter classify --input scan.json --provider ollama --model llama3
  flakehunter classify --input scan.json --provider deterministic
  flakehunter classify --input scan.json --offline --fixtures ./testdata/fixtures`,
	RunE: runClassify,
}

func runClassify(cmd *cobra.Command, _ []string) error {
	if classifyInput == "" {
		return fmt.Errorf("--input is required (the JSON written by `flakehunter scan --output`)")
	}
	if err := cfg.Validate(false); err != nil {
		return err
	}

	f, err := os.Open(classifyInput) //nolint:gosec // operator-supplied path
	if err != nil {
		return fmt.Errorf("opening %q: %w", classifyInput, err)
	}
	defer func() { _ = f.Close() }()

	var scan report.ScanResult
	if err := report.ReadJSON(f, &scan); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Minute)
	defer cancel()

	provider, err := llm.New(cfg.LLMOptions())
	if err != nil {
		return err
	}

	source, err := buildSource(ctx)
	if err != nil {
		return err
	}

	result := report.ClassifiedResult{
		Repo:        scan.Repo,
		Source:      scan.Source,
		GeneratedAt: time.Now().UTC(),
		Provider:    provider.Name(),
		Model:       provider.Model(),
		Baseline:    provider.Name() == llm.ProviderDeterministic,
	}

	if result.Baseline {
		fmt.Fprintln(os.Stderr, color.YellowString(
			"note: using the rule-based baseline provider — these verdicts are not from a language model"))
	}

	dim := color.New(color.FgHiBlack).SprintFunc()

	for i, occ := range scan.Occurrences {
		fmt.Printf("[%d/%d] %s %s\n", i+1, len(scan.Occurrences), occ.JobName, dim(occ.HeadSHA))

		item := report.Classified{Occurrence: occ}

		failure, ok := occ.PrimaryFailure()
		if !ok {
			item.Error = "occurrence has no failing run"
			result.Items = append(result.Items, item)
			continue
		}

		archive, err := source.DownloadJobLog(ctx, failure.RunID, failure.JobID)
		if err != nil {
			item.Error = fmt.Sprintf("downloading log: %v", err)
			result.Items = append(result.Items, item)
			continue
		}

		excerpt, err := extractor.ChunkArchive(archive, extractor.DefaultChunkOptions())
		if err != nil {
			item.Error = fmt.Sprintf("extracting log: %v", err)
			result.Items = append(result.Items, item)
			continue
		}

		item.StepName = excerpt.StepName
		item.ExcerptLen = excerpt.ExcerptBytes

		v, err := provider.Classify(ctx, llm.Request{
			JobName:  occ.JobName,
			StepName: excerpt.StepName,
			Excerpt:  excerpt.Text,
		})
		if err != nil {
			item.Error = fmt.Sprintf("classifying: %v", err)
			result.Items = append(result.Items, item)
			continue
		}

		item.Verdict = v.VerifyCitations(excerpt.Text).ApplyConfidenceFloor(cfg.MinConfidence)
		result.Items = append(result.Items, item)

		fmt.Printf("        %s  %s\n",
			categoryColor(string(item.Verdict.Category)),
			dim(fmt.Sprintf("confidence %.2f · %s", item.Verdict.Confidence, excerpt.StepName)))
	}

	out := os.Stdout
	if classifyOutput != "" {
		file, err := os.Create(classifyOutput) //nolint:gosec // operator-supplied path
		if err != nil {
			return fmt.Errorf("creating %q: %w", classifyOutput, err)
		}
		defer func() { _ = file.Close() }()
		out = file
	}

	if classifyOutput != "" {
		if err := report.WriteJSON(out, result); err != nil {
			return err
		}
		fmt.Printf("\nWrote %s\n", classifyOutput)
		return nil
	}

	fmt.Println()
	return report.WriteJSON(os.Stdout, result)
}

func categoryColor(category string) string {
	switch category {
	case "genuine_bug":
		return color.New(color.FgRed, color.Bold).Sprint(category)
	case "race_condition", "test_order_dependency":
		return color.New(color.FgYellow, color.Bold).Sprint(category)
	case "unknown":
		return color.New(color.FgHiBlack).Sprint(category)
	default:
		return color.New(color.FgCyan).Sprint(category)
	}
}

func init() {
	f := classifyCmd.Flags()
	f.StringVarP(&classifyInput, "input", "i", "", "scan result JSON produced by `flakehunter scan --output`")
	f.StringVarP(&classifyOutput, "output", "o", "", "write classified results to this path")
	f.BoolVar(&cfg.Offline, "offline", false, "read logs from local fixtures")
	f.StringVar(&cfg.FixturesDir, "fixtures", "", "fixture directory for --offline")
	f.StringVar(&cfg.Token, "token", "", "GitHub token (defaults to $GITHUB_TOKEN)")

	rootCmd.AddCommand(classifyCmd)
}
