package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ebenezer-isaac/llmconveyors-go-package/internal/client"
	"github.com/ebenezer-isaac/llmconveyors-go-package/internal/config"
	"github.com/ebenezer-isaac/llmconveyors-go-package/internal/output"
	"github.com/spf13/cobra"
)

var (
	// Shared client and formatter, initialized in PersistentPreRun.
	apiClient *client.Client
	formatter *output.Formatter
	cfg       config.Config
)

// ErrAwaitingInput is a sentinel error indicating the generation is paused
// waiting for user interaction. Execute() maps this to exit code 2.
var ErrAwaitingInput = errors.New("generation awaiting user input")

var rootCmd = &cobra.Command{
	Use:   "llmc",
	Short: "LLM Conveyors CLI — AI agent platform",
	Long:  "Official CLI for the LLM Conveyors AI Agent Platform.\nRun AI agents, manage sessions, score resumes, and more.",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Commands that don't need config or auth get a minimal setup.
		if commandSkipsAuth(cmd) {
			format := output.ParseFormat("text")
			if cmd.Flags().Changed("output") {
				outVal, _ := cmd.Flags().GetString("output")
				format = output.ParseFormat(outVal)
			}
			formatter = output.NewFormatter(format, os.Stdout)
			return nil
		}

		// Load config: flag > env > file > default.
		configPath, _ := cmd.Flags().GetString("config")
		var err error
		cfg, err = config.Load(configPath)
		if err != nil {
			return err
		}

		// Override config with flags if set explicitly.
		if cmd.Flags().Changed("api-key") {
			cfg.APIKey, _ = cmd.Flags().GetString("api-key")
		}
		if cmd.Flags().Changed("base-url") {
			cfg.BaseURL, _ = cmd.Flags().GetString("base-url")
		}
		if cmd.Flags().Changed("output") {
			cfg.Output, _ = cmd.Flags().GetString("output")
		}
		if cmd.Flags().Changed("debug") {
			cfg.Debug, _ = cmd.Flags().GetBool("debug")
		}
		if cmd.Flags().Changed("no-color") {
			cfg.NoColor, _ = cmd.Flags().GetBool("no-color")
		}
		if cmd.Flags().Changed("timeout") {
			cfg.Timeout, _ = cmd.Flags().GetDuration("timeout")
		}

		// Setup formatter (data → stdout).
		format := output.ParseFormat(cfg.Output)
		formatter = output.NewFormatter(format, os.Stdout)

		// Initialize API client for commands that need auth.
		if !commandSkipsClient(cmd) {
			var opts []client.Option
			if cfg.Timeout > 0 {
				opts = append(opts, client.WithTimeout(cfg.Timeout))
			}
			if cfg.Debug {
				opts = append(opts, client.WithDebug(os.Stderr))
			}
			if cfg.MaxRetries > 0 {
				opts = append(opts, client.WithMaxRetries(cfg.MaxRetries))
			}
			opts = append(opts, client.WithUserAgent("llmconveyors-go/"+versionStr))

			apiClient, err = client.New(cfg.APIKey, cfg.BaseURL, opts...)
			if err != nil {
				return err
			}
		}

		return nil
	},
	SilenceUsage:  true,
	SilenceErrors: true,
}

// commandSkipsAuth returns true for commands that need no config loading at all.
func commandSkipsAuth(cmd *cobra.Command) bool {
	path := cmd.CommandPath()
	return path == "llmc version" || path == "llmc help" ||
		strings.HasPrefix(path, "llmc config ")
}

// commandSkipsClient returns true for commands that need config/formatter but no API client.
func commandSkipsClient(cmd *cobra.Command) bool {
	path := cmd.CommandPath()
	return path == "llmc health"
}

// Execute runs the root command.
func Execute() {
	err := rootCmd.Execute()
	if err == nil {
		return
	}

	if errors.Is(err, ErrAwaitingInput) {
		os.Exit(2)
	}

	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func init() {
	rootCmd.PersistentFlags().String("api-key", "", "API key (overrides LLMC_API_KEY env var)")
	rootCmd.PersistentFlags().String("base-url", "https://api.llmconveyors.com/api/v1", "API base URL")
	rootCmd.PersistentFlags().StringP("output", "o", "text", "Output format: json, table, text")
	rootCmd.PersistentFlags().Bool("debug", false, "Enable debug logging to stderr")
	rootCmd.PersistentFlags().Bool("no-color", false, "Disable colored output")
	rootCmd.PersistentFlags().Duration("timeout", 30*time.Second, "Request timeout (e.g. 30s)")
	rootCmd.PersistentFlags().String("config", "", "Config file path (default: ~/.llmconveyors/config.yaml)")
}
