package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"

	"github.com/mdryaaan/flakehunter/internal/detector"
	ghsrc "github.com/mdryaaan/flakehunter/internal/github"
	"github.com/mdryaaan/flakehunter/internal/report"
)

var scanOutput string

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Find jobs that both passed and failed on the same commit",
	Long: `scan fetches recent workflow runs and identifies flaky occurrences.

A job counts as flaky only when the same job name, in the same workflow, on the
same commit SHA, produced both a pass and a failure. Reruns are what make this
detectable, which is why every attempt of every run is fetched rather than only
the latest.`,
	Example: `  flakehunter scan --repo acme/orders-api
  flakehunter scan --repo acme/orders-api --days 14 --workflow ci.yml
  flakehunter scan --offline --fixtures ./testdata/fixtures --output scan.json`,
	RunE: runScan,
}

func runScan(cmd *cobra.Command, _ []string) error {
	if err := cfg.Validate(true); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Minute)
	defer cancel()

	source, err := buildSource(ctx)
	if err != nil {
		return err
	}

	opts := ghsrc.ListOptions{
		Owner:        cfg.Owner,
		Repo:         cfg.RepoName,
		Days:         cfg.Days,
		WorkflowFile: cfg.WorkflowFile,
		MaxRuns:      cfg.MaxRuns,
	}

	jobs, err := source.ListJobResults(ctx, opts)
	if err != nil {
		return fmt.Errorf("fetching job results: %w", err)
	}

	occurrences := detector.Detect(jobs, detector.DefaultOptions())
	summary := detector.Summarise(jobs, occurrences)

	result := report.ScanResult{
		Repo:        repoLabel(),
		Source:      source.Describe(),
		GeneratedAt: time.Now().UTC(),
		WindowDays:  cfg.Days,
		Summary:     summary,
		Occurrences: occurrences,
	}

	if scanOutput != "" {
		f, err := os.Create(scanOutput) //nolint:gosec // operator-supplied path
		if err != nil {
			return fmt.Errorf("creating %q: %w", scanOutput, err)
		}
		defer func() { _ = f.Close() }()
		if err := report.WriteJSON(f, result); err != nil {
			return err
		}
	}

	printScan(result)

	if scanOutput != "" {
		fmt.Printf("\nWrote %s\n", scanOutput)
	}
	return nil
}

func printScan(result report.ScanResult) {
	bold := color.New(color.Bold).SprintFunc()
	dim := color.New(color.FgHiBlack).SprintFunc()

	fmt.Printf("%s  %s\n", bold(result.Repo), dim(result.Source))
	fmt.Printf("%s\n\n", dim(fmt.Sprintf("window: last %d days", result.WindowDays)))

	fmt.Printf("  %-22s %d\n", "job executions", result.Summary.TotalJobs)
	fmt.Printf("  %-22s %d\n", "distinct commits", result.Summary.DistinctCommits)
	fmt.Printf("  %-22s %d\n", "flaky occurrences", result.Summary.FlakyOccurrences)
	fmt.Printf("  %-22s %d\n\n", "affected jobs", result.Summary.AffectedJobs)

	if len(result.Occurrences) == 0 {
		fmt.Println(color.GreenString("No flaky occurrences found — every job agreed with itself."))
		return
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Job", "Commit", "Workflow", "Fail rate", "Attempts"})
	table.SetAutoWrapText(false)
	table.SetBorder(false)
	table.SetColumnSeparator("")
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)

	for _, occ := range result.Occurrences {
		sha := occ.HeadSHA
		if len(sha) > 7 {
			sha = sha[:7]
		}
		table.Append([]string{
			occ.JobName,
			sha,
			occ.WorkflowFile,
			fmt.Sprintf("%.0f%%", occ.FailureRate*100),
			fmt.Sprintf("%d", occ.TotalAttempts),
		})
	}
	table.Render()
}

func buildSource(ctx context.Context) (ghsrc.Source, error) {
	if cfg.Offline {
		return ghsrc.NewFixtureSource(cfg.FixturesDir)
	}
	return ghsrc.NewClient(ctx, cfg.Token).For(cfg.Owner, cfg.RepoName), nil
}

func repoLabel() string {
	if cfg.Repo != "" {
		return cfg.Repo
	}
	return "(offline fixtures)"
}

func init() {
	f := scanCmd.Flags()
	f.StringVar(&cfg.Repo, "repo", "", "repository in owner/name form")
	f.IntVar(&cfg.Days, "days", cfg.Days, "how many days back to scan")
	f.StringVar(&cfg.WorkflowFile, "workflow", "", "restrict to one workflow file, e.g. ci.yml")
	f.IntVar(&cfg.MaxRuns, "max-runs", cfg.MaxRuns, "cap on workflow runs fetched")
	f.BoolVar(&cfg.Offline, "offline", false, "read from local fixtures instead of the GitHub API")
	f.StringVar(&cfg.FixturesDir, "fixtures", "", "fixture directory for --offline")
	f.StringVar(&cfg.Token, "token", "", "GitHub token (defaults to $GITHUB_TOKEN)")
	f.StringVarP(&scanOutput, "output", "o", "", "write the scan result as JSON to this path")

	rootCmd.AddCommand(scanCmd)
}
