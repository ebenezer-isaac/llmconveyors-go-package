package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/llmconveyors/cli/internal/output"
	"github.com/spf13/cobra"
)

var apiKeysCmd = &cobra.Command{
	Use:   "api-keys",
	Short: "Manage platform API keys",
}

var apiKeysListCmd = &cobra.Command{
	Use:   "list",
	Short: "List API keys",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		var result []struct {
			Hash      string   `json:"hash"`
			Name      string   `json:"name"`
			Scopes    []string `json:"scopes"`
			CreatedAt string   `json:"createdAt"`
		}
		if err := apiClient.Get(ctx, "/settings/platform-api-keys", &result); err != nil {
			return err
		}

		switch formatter.Format {
		case output.FormatJSON:
			return formatter.WriteJSON(result)
		case output.FormatTable:
			rows := make([][]string, len(result))
			for i, k := range result {
				rows[i] = []string{k.Hash, k.Name, strings.Join(k.Scopes, ","), k.CreatedAt}
			}
			return formatter.WriteTable([]string{"HASH", "NAME", "SCOPES", "CREATED"}, rows)
		default:
			for _, k := range result {
				formatter.WriteText("", []output.KeyValue{
					{Key: "Hash", Value: k.Hash},
					{Key: "Name", Value: k.Name},
					{Key: "Scopes", Value: strings.Join(k.Scopes, ", ")},
					{Key: "Created", Value: k.CreatedAt},
				})
				fmt.Fprintln(os.Stdout)
			}
			return nil
		}
	},
}

var apiKeysCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new API key",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		name, _ := cmd.Flags().GetString("name")
		scopes, _ := cmd.Flags().GetString("scopes")

		if name == "" {
			return fmt.Errorf("--name is required")
		}

		body := map[string]interface{}{
			"name": name,
		}
		if scopes != "" {
			raw := strings.Split(scopes, ",")
			cleaned := make([]string, 0, len(raw))
			for _, s := range raw {
				s = strings.TrimSpace(s)
				if s != "" {
					cleaned = append(cleaned, s)
				}
			}
			if len(cleaned) > 0 {
				body["scopes"] = cleaned
			}
		}

		var result struct {
			Key  string `json:"key"`
			Hash string `json:"hash"`
		}
		if err := apiClient.Post(ctx, "/settings/platform-api-keys", body, &result); err != nil {
			return err
		}

		switch formatter.Format {
		case output.FormatJSON:
			return formatter.WriteJSON(result)
		default:
			return formatter.WriteText("API Key Created (save this — it won't be shown again)", []output.KeyValue{
				{Key: "Key", Value: result.Key},
				{Key: "Hash", Value: result.Hash},
			})
		}
	},
}

var apiKeysRevokeCmd = &cobra.Command{
	Use:   "revoke <hash>",
	Short: "Revoke an API key",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		if err := apiClient.Delete(ctx, "/settings/platform-api-keys/"+args[0], nil); err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "API key %s revoked\n", args[0])
		return nil
	},
}

var apiKeysRotateCmd = &cobra.Command{
	Use:   "rotate <hash>",
	Short: "Rotate an API key",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		var result struct {
			NewKey string `json:"newKey"`
		}
		if err := apiClient.Post(ctx, "/settings/platform-api-keys/"+args[0]+"/rotate", nil, &result); err != nil {
			return err
		}

		switch formatter.Format {
		case output.FormatJSON:
			return formatter.WriteJSON(result)
		default:
			return formatter.WriteText("API Key Rotated (save this — it won't be shown again)", []output.KeyValue{
				{Key: "New Key", Value: result.NewKey},
			})
		}
	},
}

var apiKeysUsageCmd = &cobra.Command{
	Use:   "usage <hash>",
	Short: "Show usage statistics for an API key",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		var result map[string]interface{}
		if err := apiClient.Get(ctx, "/settings/platform-api-keys/"+args[0]+"/usage", &result); err != nil {
			return err
		}

		return formatter.WriteJSON(result)
	},
}

func init() {
	apiKeysCreateCmd.Flags().String("name", "", "Key name (required)")
	apiKeysCreateCmd.Flags().String("scopes", "", "Comma-separated scopes")

	apiKeysCmd.AddCommand(apiKeysListCmd)
	apiKeysCmd.AddCommand(apiKeysCreateCmd)
	apiKeysCmd.AddCommand(apiKeysRevokeCmd)
	apiKeysCmd.AddCommand(apiKeysRotateCmd)
	apiKeysCmd.AddCommand(apiKeysUsageCmd)
	rootCmd.AddCommand(apiKeysCmd)
}
