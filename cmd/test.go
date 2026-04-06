package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	fixture "github.com/dakshpathak/minder-rule-testing/pkg/testing"
)

func NewTestCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test [fixture-file]",
		Short: "Run rule tests from a YAML fixture file",
		Long: `Parses a YAML fixture file, builds offline mocks for each test case,
and validates the fixture structure. In the full Minder integration this
will fire up the rule engine and evaluate each case against the rule type.

For now, it runs DryRun validation which confirms:
  - The YAML schema is correct
  - All HTTP URLs are absolute
  - Data source keys follow the "name.func" format
  - Git mock files are well formed
  - All mock layers build without error`,
		Args: cobra.ExactArgs(1),
		RunE: runTest,
	}

	return cmd
}

func NewDryRunCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dryrun [fixture-file]",
		Short: "Validate a fixture file without running the engine",
		Long: `Checks the fixture YAML for structural correctness and validates all
mock data. This is the fast pre-check that CI runs on every pull request
before the full rule evaluation suite.`,
		Args: cobra.ExactArgs(1),
		RunE: runDryRun,
	}

	return cmd
}

func runTest(cmd *cobra.Command, args []string) error {
	fixturePath := args[0]

	if _, err := os.Stat(fixturePath); err != nil {
		return fmt.Errorf("cannot find fixture file: %v", err)
	}

	fmt.Printf("Loading fixture: %s\n", fixturePath)

	results, err := fixture.Evaluate(fixturePath)
	if err != nil {
		return fmt.Errorf("evaluation failed: %v", err)
	}

	printResults(results)
	return nil
}

func runDryRun(cmd *cobra.Command, args []string) error {
	fixturePath := args[0]

	if _, err := os.Stat(fixturePath); err != nil {
		return fmt.Errorf("cannot find fixture file: %v", err)
	}

	fmt.Printf("Validating fixture: %s\n", fixturePath)

	results, err := fixture.DryRun(fixturePath)
	if err != nil {
		return fmt.Errorf("validation failed: %v", err)
	}

	printResults(results)
	return nil
}

func printResults(results []fixture.Result) {
	passed := 0
	failed := 0
	skipped := 0

	for _, r := range results {
		switch {
		case r.Skipped:
			fmt.Printf("  SKIP  %s (%s)\n", r.Name, r.SkipReason)
			skipped++
		case r.Err != nil:
			fmt.Printf("  FAIL  %s: %v\n", r.Name, r.Err)
			failed++
		default:
			fmt.Printf("  PASS  %s\n", r.Name)
			passed++
		}
	}

	fmt.Printf("\nResults: %d passed, %d failed, %d skipped\n", passed, failed, skipped)
}
