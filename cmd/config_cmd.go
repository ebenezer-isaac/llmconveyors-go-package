package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ebenezer-isaac/llmconveyors-go-package/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage CLI configuration",
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create config file interactively",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := config.EnsureConfigDir()
		if err != nil {
			return err
		}

		path := filepath.Join(dir, "config.yaml")
		if _, err := os.Stat(path); err == nil {
			fmt.Fprintf(os.Stderr, "Config file already exists at %s\n", path)
			fmt.Fprint(os.Stderr, "Overwrite? [y/N]: ")
			reader := bufio.NewReader(os.Stdin)
			answer, _ := reader.ReadString('\n')
			if strings.TrimSpace(strings.ToLower(answer)) != "y" {
				fmt.Fprintln(os.Stderr, "Aborted.")
				return nil
			}
		}

		reader := bufio.NewReader(os.Stdin)

		fmt.Fprint(os.Stderr, "API Key (llmc_...): ")
		apiKey, _ := reader.ReadString('\n')
		apiKey = strings.TrimSpace(apiKey)

		if apiKey != "" && (len(apiKey) < 6 || apiKey[:5] != "llmc_") {
			return fmt.Errorf("API key must start with 'llmc_' prefix")
		}

		fmt.Fprint(os.Stderr, "Base URL [https://api.llmconveyors.com/api/v1]: ")
		baseURL, _ := reader.ReadString('\n')
		baseURL = strings.TrimSpace(baseURL)
		if baseURL == "" {
			baseURL = "https://api.llmconveyors.com/api/v1"
		}

		fmt.Fprint(os.Stderr, "Output format (text/json/table) [text]: ")
		outputFmt, _ := reader.ReadString('\n')
		outputFmt = strings.TrimSpace(outputFmt)
		if outputFmt == "" {
			outputFmt = "text"
		}

		values := map[string]interface{}{
			"api_key":     apiKey,
			"base_url":    baseURL,
			"output":      outputFmt,
			"timeout":     "30s",
			"max_retries": 3,
			"debug":       false,
			"no_color":    false,
		}

		if err := config.WriteConfigFile(path, values); err != nil {
			return fmt.Errorf("writing config file: %w", err)
		}

		fmt.Fprintf(os.Stderr, "Config written to %s\n", path)
		return nil
	},
}

// resolveConfigPath returns the config file path from the --config flag or the default.
func resolveConfigPath(cmd *cobra.Command) string {
	// Walk up to root to find the persistent --config flag.
	configPath, _ := cmd.Root().Flags().GetString("config")
	if configPath != "" {
		return configPath
	}
	return config.DefaultConfigPath()
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a config value",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		value := args[1]

		configPath := resolveConfigPath(cmd)

		// Read existing config.
		v := viper.New()
		v.SetConfigFile(configPath)
		v.SetConfigType("yaml")
		_ = v.ReadInConfig() // OK if file doesn't exist yet.

		v.Set(key, value)

		// Ensure directory exists.
		if _, err := config.EnsureConfigDir(); err != nil {
			return err
		}

		if err := v.WriteConfigAs(configPath); err != nil {
			return fmt.Errorf("writing config: %w", err)
		}

		fmt.Fprintf(os.Stderr, "Set %s = %s\n", key, value)
		return nil
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a config value",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]

		configPath := resolveConfigPath(cmd)

		v := viper.New()
		v.SetConfigFile(configPath)
		v.SetConfigType("yaml")
		if err := v.ReadInConfig(); err != nil {
			return fmt.Errorf("reading config: %w", err)
		}

		val := v.Get(key)
		if val == nil {
			return fmt.Errorf("key %q not found in config", key)
		}

		fmt.Println(val)
		return nil
	},
}

func init() {
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configGetCmd)
	rootCmd.AddCommand(configCmd)
}
