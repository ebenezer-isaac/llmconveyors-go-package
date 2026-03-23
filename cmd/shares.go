package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var sharesCmd = &cobra.Command{
	Use:   "shares",
	Short: "Manage public share links",
}

var sharesCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a public share link for a generated document",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		dataStr, _ := cmd.Flags().GetString("data")

		if dataStr == "" {
			return fmt.Errorf("--data is required (JSON body)")
		}

		var body interface{}
		if err := json.Unmarshal([]byte(dataStr), &body); err != nil {
			return fmt.Errorf("invalid --data JSON: %w", err)
		}

		var result map[string]interface{}
		if err := apiClient.Post(ctx, "/shares", body, &result); err != nil {
			return err
		}

		return formatter.WriteJSON(result)
	},
}

var sharesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all share links created by the authenticated user",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		var result map[string]interface{}
		if err := apiClient.Get(ctx, "/shares/stats", &result); err != nil {
			return err
		}

		return formatter.WriteJSON(result)
	},
}

var sharesGetPublicCmd = &cobra.Command{
	Use:   "get <slug>",
	Short: "Get public share data (no auth required)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		var result map[string]interface{}
		if err := apiClient.Get(ctx, "/shares/"+args[0]+"/public", &result); err != nil {
			return err
		}

		return formatter.WriteJSON(result)
	},
}

var sharesVisitCmd = &cobra.Command{
	Use:   "visit <slug>",
	Short: "Record a visit to a shared document",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		if err := apiClient.Post(ctx, "/shares/"+args[0]+"/visit", nil, nil); err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "Visit recorded for share %s\n", args[0])
		return nil
	},
}

var sharesStatsCmd = &cobra.Command{
	Use:   "stats <slug>",
	Short: "Get visit statistics for a share link",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		var result map[string]interface{}
		if err := apiClient.Get(ctx, "/shares/"+args[0]+"/stats", &result); err != nil {
			return err
		}

		return formatter.WriteJSON(result)
	},
}

func init() {
	sharesCreateCmd.Flags().String("data", "", "JSON body for share creation")

	sharesCmd.AddCommand(sharesCreateCmd)
	sharesCmd.AddCommand(sharesListCmd)
	sharesCmd.AddCommand(sharesGetPublicCmd)
	sharesCmd.AddCommand(sharesVisitCmd)
	sharesCmd.AddCommand(sharesStatsCmd)
	rootCmd.AddCommand(sharesCmd)
}
