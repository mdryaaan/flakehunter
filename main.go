// Command flakehunter finds flaky tests in GitHub Actions CI, classifies why
// they are flaky, and reports what to do about them.
package main

import (
	"fmt"
	"os"

	"github.com/mdryaaan/flakehunter/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "flakehunter: %v\n", err)
		os.Exit(1)
	}
}
