package cmd

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

// --- Auth commands (GDPR export, account deletion) ---

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authentication and account management",
}

var authExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export all user data (GDPR)",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		dest, _ := cmd.Flags().GetString("dest")

		resp, err := apiClient.GetRaw(ctx, "/auth/export")
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		var w io.Writer
		if dest != "" {
			f, err := os.Create(dest)
			if err != nil {
				return fmt.Errorf("creating output file: %w", err)
			}
			defer f.Close()
			w = f
		} else {
			w = os.Stdout
		}

		n, err := io.Copy(w, resp.Body)
		if err != nil {
			return fmt.Errorf("downloading export: %w", err)
		}

		if dest != "" {
			fmt.Fprintf(os.Stderr, "Exported %d bytes to %s\n", n, dest)
		}

		return nil
	},
}

var authDeleteAccountCmd = &cobra.Command{
	Use:   "delete-account",
	Short: "Permanently delete your account and all data",
	RunE: func(cmd *cobra.Command, args []string) error {
		confirm, _ := cmd.Flags().GetBool("confirm")
		if !confirm {
			return fmt.Errorf("pass --confirm to permanently delete your account (this cannot be undone)")
		}

		ctx := context.Background()
		if err := apiClient.Delete(ctx, "/auth/account", nil); err != nil {
			return err
		}

		fmt.Fprintln(os.Stderr, "Account deleted permanently")
		return nil
	},
}

// --- Privacy / Consent commands ---

var privacyCmd = &cobra.Command{
	Use:   "privacy",
	Short: "Manage privacy consents",
}

var privacyConsentsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all consent records",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		var result map[string]interface{}
		if err := apiClient.Get(ctx, "/privacy/consents", &result); err != nil {
			return err
		}

		return formatter.WriteJSON(result)
	},
}

var privacyConsentsGrantCmd = &cobra.Command{
	Use:   "grant <purpose>",
	Short: "Grant consent for a purpose",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		var result map[string]interface{}
		if err := apiClient.Post(ctx, "/privacy/consents/"+args[0], nil, &result); err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "Consent granted for %q\n", args[0])
		return nil
	},
}

var privacyConsentsWithdrawCmd = &cobra.Command{
	Use:   "withdraw <purpose>",
	Short: "Withdraw consent for a purpose",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		if err := apiClient.Delete(ctx, "/privacy/consents/"+args[0], nil); err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "Consent withdrawn for %q\n", args[0])
		return nil
	},
}

func init() {
	authExportCmd.Flags().String("dest", "", "Output file path for export")
	authDeleteAccountCmd.Flags().Bool("confirm", false, "Confirm account deletion (required)")

	authCmd.AddCommand(authExportCmd)
	authCmd.AddCommand(authDeleteAccountCmd)

	privacyCmd.AddCommand(privacyConsentsListCmd)
	privacyCmd.AddCommand(privacyConsentsGrantCmd)
	privacyCmd.AddCommand(privacyConsentsWithdrawCmd)

	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(privacyCmd)
}
