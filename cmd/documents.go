package cmd

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

var documentsCmd = &cobra.Command{
	Use:   "documents",
	Short: "Manage generated documents",
}

var documentsDownloadCmd = &cobra.Command{
	Use:   "download",
	Short: "Download a generated document",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		key, _ := cmd.Flags().GetString("key")
		dest, _ := cmd.Flags().GetString("dest")

		if key == "" {
			return fmt.Errorf("--key is required")
		}

		path := fmt.Sprintf("/documents/download?key=%s", key)
		resp, err := apiClient.GetRaw(ctx, path)
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
			return fmt.Errorf("downloading: %w", err)
		}

		if dest != "" {
			fmt.Fprintf(os.Stderr, "Downloaded %d bytes to %s\n", n, dest)
		}

		return nil
	},
}

func init() {
	documentsDownloadCmd.Flags().String("key", "", "Document key to download")
	documentsDownloadCmd.Flags().String("dest", "", "Output file path")

	documentsCmd.AddCommand(documentsDownloadCmd)
	rootCmd.AddCommand(documentsCmd)
}
