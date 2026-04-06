package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

func rootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "minder-test",
		Short: "Offline rule testing tool for Minder",
		Long: `A standalone testing tool that lets you validate Minder security rules
without network access. Write a single YAML fixture file, run the test,
and get instant feedback on whether your rule behaves correctly.

Supports REST API rules, Git file rules, and Data Source (Rego) rules
through three mock layers that intercept the rule engine at each level.`,
	}

	cmd.AddCommand(NewTestCommand())
	cmd.AddCommand(NewDryRunCommand())

	return cmd
}

func Execute() {
	if err := rootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
