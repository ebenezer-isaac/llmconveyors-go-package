package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/llmconveyors/cli/internal/output"
	"github.com/spf13/cobra"
)

var settingsCmd = &cobra.Command{
	Use:   "settings",
	Short: "Manage user settings",
}

var settingsProfileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Show user profile",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		var result struct {
			Credits       float64 `json:"credits"`
			Tier          string  `json:"tier"`
			ByoKeyEnabled bool    `json:"byoKeyEnabled"`
		}
		if err := apiClient.Get(ctx, "/settings/profile", &result); err != nil {
			return err
		}

		switch formatter.Format {
		case output.FormatJSON:
			return formatter.WriteJSON(result)
		default:
			return formatter.WriteText("Profile", []output.KeyValue{
				{Key: "Credits", Value: fmt.Sprintf("%.2f", result.Credits)},
				{Key: "Tier", Value: result.Tier},
				{Key: "BYO Key", Value: fmt.Sprintf("%v", result.ByoKeyEnabled)},
			})
		}
	},
}

var settingsPreferencesCmd = &cobra.Command{
	Use:   "preferences",
	Short: "Get or set preferences",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		setValues, _ := cmd.Flags().GetStringArray("set")

		if len(setValues) > 0 {
			body := map[string]interface{}{}
			for _, kv := range setValues {
				parts := strings.SplitN(kv, "=", 2)
				if len(parts) != 2 {
					return fmt.Errorf("invalid format %q, expected key=value", kv)
				}
				body[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}

			var result map[string]interface{}
			if err := apiClient.Post(ctx, "/settings/preferences", body, &result); err != nil {
				return err
			}

			fmt.Fprintln(os.Stderr, "Preferences updated")
			return formatter.WriteJSON(result)
		}

		var result map[string]interface{}
		if err := apiClient.Get(ctx, "/settings/preferences", &result); err != nil {
			return err
		}

		return formatter.WriteJSON(result)
	},
}

var settingsUsageCmd = &cobra.Command{
	Use:   "usage",
	Short: "Show credit usage summary",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		var result struct {
			TotalCreditsUsed           float64 `json:"totalCreditsUsed"`
			TotalGenerations           int     `json:"totalGenerations"`
			AverageCreditsPerGeneration float64 `json:"averageCreditsPerGeneration"`
		}
		if err := apiClient.Get(ctx, "/settings/usage-summary", &result); err != nil {
			return err
		}

		switch formatter.Format {
		case output.FormatJSON:
			return formatter.WriteJSON(result)
		default:
			return formatter.WriteText("Usage Summary", []output.KeyValue{
				{Key: "Total Credits", Value: fmt.Sprintf("%.2f", result.TotalCreditsUsed)},
				{Key: "Generations", Value: fmt.Sprintf("%d", result.TotalGenerations)},
				{Key: "Avg Credits/Gen", Value: fmt.Sprintf("%.2f", result.AverageCreditsPerGeneration)},
			})
		}
	},
}

var settingsUsageLogsCmd = &cobra.Command{
	Use:   "usage-logs",
	Short: "Show detailed usage logs",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		limit, _ := cmd.Flags().GetInt("limit")
		offset, _ := cmd.Flags().GetInt("offset")

		path := fmt.Sprintf("/settings/usage-logs?limit=%d&offset=%d", limit, offset)

		var result map[string]interface{}
		if err := apiClient.Get(ctx, path, &result); err != nil {
			return err
		}

		return formatter.WriteJSON(result)
	},
}

// --- BYO API Key Management ---

var settingsByoKeyCmd = &cobra.Command{
	Use:   "byo-key",
	Short: "Manage BYO (Bring Your Own) API key",
}

var settingsByoKeyGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Check if BYO API key is configured",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		var result map[string]interface{}
		if err := apiClient.Get(ctx, "/settings/api-key", &result); err != nil {
			return err
		}
		return formatter.WriteJSON(result)
	},
}

var settingsByoKeySetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set BYO API key",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		key, _ := cmd.Flags().GetString("key")
		if key == "" {
			return fmt.Errorf("--key is required")
		}
		body := map[string]interface{}{"apiKey": key}
		var result map[string]interface{}
		if err := apiClient.Post(ctx, "/settings/api-key", body, &result); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "BYO API key set")
		return nil
	},
}

var settingsByoKeyDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Remove BYO API key",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		if err := apiClient.Delete(ctx, "/settings/api-key", nil); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "BYO API key removed")
		return nil
	},
}

// --- Webhook Secret Management ---

var settingsWebhookCmd = &cobra.Command{
	Use:   "webhook-secret",
	Short: "Manage webhook signing secret",
}

var settingsWebhookGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get or create webhook secret",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		var result map[string]interface{}
		if err := apiClient.Get(ctx, "/settings/webhook-secret", &result); err != nil {
			return err
		}
		return formatter.WriteJSON(result)
	},
}

var settingsWebhookRotateCmd = &cobra.Command{
	Use:   "rotate",
	Short: "Rotate webhook secret",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		var result map[string]interface{}
		if err := apiClient.Post(ctx, "/settings/webhook-secret/rotate", nil, &result); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "Webhook secret rotated")
		return formatter.WriteJSON(result)
	},
}

func init() {
	settingsPreferencesCmd.Flags().StringArray("set", nil, "Set preference (key=value, repeatable)")

	settingsUsageLogsCmd.Flags().Int("limit", 50, "Number of logs to fetch")
	settingsUsageLogsCmd.Flags().Int("offset", 0, "Offset for pagination")

	settingsByoKeySetCmd.Flags().String("key", "", "BYO API key value")

	settingsByoKeyCmd.AddCommand(settingsByoKeyGetCmd)
	settingsByoKeyCmd.AddCommand(settingsByoKeySetCmd)
	settingsByoKeyCmd.AddCommand(settingsByoKeyDeleteCmd)

	settingsWebhookCmd.AddCommand(settingsWebhookGetCmd)
	settingsWebhookCmd.AddCommand(settingsWebhookRotateCmd)

	settingsCmd.AddCommand(settingsProfileCmd)
	settingsCmd.AddCommand(settingsPreferencesCmd)
	settingsCmd.AddCommand(settingsUsageCmd)
	settingsCmd.AddCommand(settingsUsageLogsCmd)
	settingsCmd.AddCommand(settingsByoKeyCmd)
	settingsCmd.AddCommand(settingsWebhookCmd)
	rootCmd.AddCommand(settingsCmd)
}
