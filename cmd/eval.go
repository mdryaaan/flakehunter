package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"

	"github.com/mdryaaan/flakehunter/internal/eval"
	"github.com/mdryaaan/flakehunter/internal/llm"
	"github.com/mdryaaan/flakehunter/internal/report"
)

var (
	evalCorpus  string
	evalVerbose bool
	evalJSON    string
)

var evalCmd = &cobra.Command{
	Use:   "eval",
	Short: "Measure classifier accuracy against the labelled corpus",
	Long: `eval runs the configured provider against a hand-labelled set of CI failure
excerpts and reports accuracy, per-category precision and recall, and a
confusion matrix.

This exists because a classifier without a measured accuracy is a demo. Numbers
here are what make it possible to say whether a prompt change helped, whether a
smaller model is good enough, and whether the language model is beating the
rule-based baseline it has to justify itself against.`,
	Example: `  flakehunter eval --provider deterministic
  flakehunter eval --provider ollama --model llama3 --verbose
  flakehunter eval --provider claude --model claude-sonnet-4-6`,
	RunE: runEval,
}

func runEval(cmd *cobra.Command, _ []string) error {
	if err := cfg.Validate(false); err != nil {
		return err
	}

	corpus, err := eval.LoadCorpus(evalCorpus)
	if err != nil {
		return err
	}

	provider, err := llm.New(cfg.LLMOptions())
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Minute)
	defer cancel()

	fmt.Printf("Evaluating %s/%s against %d labelled cases\n\n",
		provider.Name(), provider.Model(), len(corpus.Cases))

	if provider.Name() == llm.ProviderDeterministic {
		fmt.Println(color.YellowString(
			"These are BASELINE numbers from the rule-based classifier.\n" +
				"No language model was involved. Use them as the floor an LLM must beat,\n" +
				"never as a measurement of model accuracy.\n"))
	}

	onCase := func(o eval.Outcome) {
		if !evalVerbose {
			return
		}
		mark := color.GreenString("PASS")
		if o.Error != "" {
			mark = color.MagentaString("ERR ")
		} else if !o.Correct {
			mark = color.RedString("FAIL")
		}
		fmt.Printf("  %s %s  expected=%-22s predicted=%-22s conf=%.2f %s\n",
			mark, o.CaseID, o.Expected, o.Predicted, o.Confidence, o.Error)
	}

	scores, err := eval.Run(ctx, corpus, provider, cfg.MinConfidence, onCase)
	if err != nil {
		return err
	}

	if evalVerbose {
		fmt.Println()
	}
	printScores(scores)

	if evalJSON != "" {
		f, err := os.Create(evalJSON) //nolint:gosec // operator-supplied path
		if err != nil {
			return fmt.Errorf("creating %q: %w", evalJSON, err)
		}
		defer func() { _ = f.Close() }()
		if err := report.WriteJSON(f, scores); err != nil {
			return err
		}
		fmt.Printf("\nWrote %s\n", evalJSON)
	}

	return nil
}

func printScores(s eval.Scores) {
	bold := color.New(color.Bold).SprintFunc()

	fmt.Println(bold("Overall"))
	fmt.Printf("  cases           %d\n", s.Total)
	fmt.Printf("  correct         %d\n", s.Correct)
	if s.Errored > 0 {
		fmt.Printf("  errored         %d\n", s.Errored)
	}
	fmt.Printf("  accuracy        %.1f%%\n", s.Accuracy*100)
	fmt.Printf("  macro F1        %.3f\n", s.MacroF1)
	fmt.Printf("  mean confidence %.2f when right, %.2f when wrong\n",
		s.MeanConfidenceCorrect, s.MeanConfidenceWrong)
	fmt.Printf("  fabricated citations dropped: %d\n\n", s.HallucinatedCitations)

	fmt.Println(bold("Per category"))
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Category", "Support", "Predicted", "Precision", "Recall", "F1"})
	table.SetBorder(false)
	table.SetColumnSeparator("")
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)

	for _, c := range s.PerCategory {
		table.Append([]string{
			string(c.Category),
			fmt.Sprintf("%d", c.Support),
			fmt.Sprintf("%d", c.Predicted),
			fmt.Sprintf("%.2f", c.Precision),
			fmt.Sprintf("%.2f", c.Recall),
			fmt.Sprintf("%.2f", c.F1),
		})
	}
	table.Render()

	fmt.Println()
	fmt.Println(bold("Confusion matrix"))
	fmt.Println(s.ConfusionMatrix())
}

func init() {
	f := evalCmd.Flags()
	f.StringVar(&evalCorpus, "corpus", "testdata/eval/labeled-cases.json", "labelled case file")
	f.BoolVarP(&evalVerbose, "verbose", "v", false, "print each case result")
	f.StringVar(&evalJSON, "json", "", "also write scores as JSON to this path")

	rootCmd.AddCommand(evalCmd)
}
