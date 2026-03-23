package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var logCmd = &cobra.Command{
	Use:   "log",
	Short: "Forward a structured log entry to the platform",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		dataStr, _ := cmd.Flags().GetString("data")

		if dataStr == "" {
			return fmt.Errorf("--data is required (JSON log payload)")
		}

		var body interface{}
		if err := json.Unmarshal([]byte(dataStr), &body); err != nil {
			return fmt.Errorf("invalid --data JSON: %w", err)
		}

		if err := apiClient.Post(ctx, "/log", body, nil); err != nil {
			return err
		}

		return nil
	},
}

func init() {
	logCmd.Flags().String("data", "", "JSON log payload")
	rootCmd.AddCommand(logCmd)
}
