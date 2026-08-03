package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/mdryaaan/flakehunter/internal/report"
)

var (
	reportInput  string
	reportOutput string
	reportFormat string
	reportSince  int
)

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Render classified results as markdown, JSON, an issue body, or a digest",
	Example: `  flakehunter report --input classified.json --format markdown
  flakehunter report --input classified.json --format issue --output issue-body.md
  flakehunter report --input classified.json --format digest --since 7`,
	RunE: runReport,
}

func runReport(_ *cobra.Command, _ []string) error {
	if reportInput == "" {
		return fmt.Errorf("--input is required (the JSON written by `flakehunter classify --output`)")
	}

	f, err := os.Open(reportInput) //nolint:gosec // operator-supplied path
	if err != nil {
		return fmt.Errorf("opening %q: %w", reportInput, err)
	}
	defer func() { _ = f.Close() }()

	var classified report.ClassifiedResult
	if err := report.ReadJSON(f, &classified); err != nil {
		return err
	}

	var out io.Writer = os.Stdout
	if reportOutput != "" {
		file, err := os.Create(reportOutput) //nolint:gosec // operator-supplied path
		if err != nil {
			return fmt.Errorf("creating %q: %w", reportOutput, err)
		}
		defer func() { _ = file.Close() }()
		out = file
	}

	switch reportFormat {
	case "markdown", "md":
		err = report.WriteMarkdown(out, classified)
	case "json":
		err = report.WriteJSON(out, classified)
	case "issue":
		err = report.WriteIssue(out, classified)
	case "digest":
		err = report.WriteDigest(out, classified, reportSince)
	default:
		return fmt.Errorf("unknown --format %q (want markdown, json, issue, or digest)", reportFormat)
	}
	if err != nil {
		return err
	}

	if reportOutput != "" {
		fmt.Printf("Wrote %s\n", reportOutput)
	}
	return nil
}

func init() {
	f := reportCmd.Flags()
	f.StringVarP(&reportInput, "input", "i", "", "classified result JSON")
	f.StringVarP(&reportOutput, "output", "o", "", "write to this path instead of stdout")
	f.StringVarP(&reportFormat, "format", "f", "markdown", "markdown, json, issue, or digest")
	f.IntVar(&reportSince, "since", 7, "window in days, shown in the digest header")

	rootCmd.AddCommand(reportCmd)
}
