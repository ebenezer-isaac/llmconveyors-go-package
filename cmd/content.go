package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var contentCmd = &cobra.Command{
	Use:   "content",
	Short: "Manage source documents and generations",
}

var contentSaveCmd = &cobra.Command{
	Use:   "save",
	Short: "Save a source document",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		filePath, _ := cmd.Flags().GetString("file")

		data, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("reading file: %w", err)
		}

		var body interface{}
		if err := json.Unmarshal(data, &body); err != nil {
			return fmt.Errorf("invalid JSON in %s: %w", filePath, err)
		}

		var result map[string]interface{}
		if err := apiClient.Post(ctx, "/content/save", body, &result); err != nil {
			return err
		}

		return formatter.WriteJSON(result)
	},
}

var contentDeleteGenCmd = &cobra.Command{
	Use:   "delete-generation <id>",
	Short: "Delete a generation",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		if err := apiClient.Delete(ctx, "/content/generations/"+args[0], nil); err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "Generation %s deleted\n", args[0])
		return nil
	},
}

func init() {
	contentSaveCmd.Flags().String("file", "", "Path to JSON document file")
	_ = contentSaveCmd.MarkFlagRequired("file")

	contentCmd.AddCommand(contentSaveCmd)
	contentCmd.AddCommand(contentDeleteGenCmd)
	rootCmd.AddCommand(contentCmd)
}
