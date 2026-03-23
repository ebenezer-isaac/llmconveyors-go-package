package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/llmconveyors/cli/internal/output"
	"github.com/spf13/cobra"
)

var referralCmd = &cobra.Command{
	Use:   "referral",
	Short: "Manage referral codes and statistics",
}

var referralStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show referral statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		var result map[string]interface{}
		if err := apiClient.Get(ctx, "/referral/stats", &result); err != nil {
			return err
		}

		return formatter.WriteJSON(result)
	},
}

var referralCodeCmd = &cobra.Command{
	Use:   "code",
	Short: "Get your referral code",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		var result struct {
			Code string `json:"code"`
		}
		if err := apiClient.Get(ctx, "/referral/code", &result); err != nil {
			return err
		}

		switch formatter.Format {
		case output.FormatJSON:
			return formatter.WriteJSON(result)
		default:
			fmt.Fprintln(os.Stdout, result.Code)
			return nil
		}
	},
}

var referralVanityCmd = &cobra.Command{
	Use:   "set-vanity <code>",
	Short: "Set a custom vanity referral code",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		body := map[string]interface{}{
			"vanityCode": args[0],
		}

		var result map[string]interface{}
		if err := apiClient.Post(ctx, "/referral/vanity-code", body, &result); err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "Vanity code set to %q\n", args[0])
		return formatter.WriteJSON(result)
	},
}

func init() {
	referralCmd.AddCommand(referralStatsCmd)
	referralCmd.AddCommand(referralCodeCmd)
	referralCmd.AddCommand(referralVanityCmd)
	rootCmd.AddCommand(referralCmd)
}
