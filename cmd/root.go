package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "llmc",
	Short: "LLM Conveyors CLI — AI agent platform",
	Long:  "Official CLI for the LLM Conveyors AI Agent Platform.\nRun AI agents, manage sessions, score resumes, and more.",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().String("api-key", "", "API key (overrides LLMC_API_KEY env var)")
	rootCmd.PersistentFlags().String("base-url", "https://api.llmconveyors.com/api/v1", "API base URL")
	rootCmd.PersistentFlags().StringP("output", "o", "text", "Output format: json, table, text")
	rootCmd.PersistentFlags().Bool("debug", false, "Enable debug logging to stderr")
	rootCmd.PersistentFlags().Bool("no-color", false, "Disable colored output")
	rootCmd.PersistentFlags().Duration("timeout", 30_000_000_000, "Request timeout (e.g. 30s)")
	rootCmd.PersistentFlags().String("config", "", "Config file path (default: ~/.llmconveyors/config.yaml)")
}
