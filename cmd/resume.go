package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/ebenezer-isaac/llmconveyors-go-package/internal/output"
	"github.com/spf13/cobra"
)

type masterResume struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

var resumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Manage master resumes",
}

var resumeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List master resumes",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		var result struct {
			Masters []masterResume `json:"masters"`
		}
		if err := apiClient.Get(ctx, "/resume/master", &result); err != nil {
			return err
		}

		switch formatter.Format {
		case output.FormatJSON:
			return formatter.WriteJSON(result)
		case output.FormatTable:
			rows := make([][]string, len(result.Masters))
			for i, m := range result.Masters {
				rows[i] = []string{m.ID, m.Name, m.CreatedAt}
			}
			return formatter.WriteTable([]string{"ID", "NAME", "CREATED"}, rows)
		default:
			for _, m := range result.Masters {
				formatter.WriteText("", []output.KeyValue{
					{Key: "ID", Value: m.ID},
					{Key: "Name", Value: m.Name},
					{Key: "Created", Value: m.CreatedAt},
				})
				fmt.Fprintln(os.Stdout)
			}
			return nil
		}
	},
}

var resumeGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get master resume",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		var result map[string]interface{}
		if err := apiClient.Get(ctx, "/resume/master/"+args[0], &result); err != nil {
			return err
		}

		return formatter.WriteJSON(result)
	},
}

var resumeCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create master resume",
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
		if err := apiClient.Post(ctx, "/resume/master", body, &result); err != nil {
			return err
		}

		switch formatter.Format {
		case output.FormatJSON:
			return formatter.WriteJSON(result)
		default:
			fmt.Fprintln(os.Stderr, "Resume created successfully")
			return formatter.WriteJSON(result)
		}
	},
}

var resumeUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update master resume",
	Args:  cobra.ExactArgs(1),
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
		if err := apiClient.Put(ctx, "/resume/master/"+args[0], body, &result); err != nil {
			return err
		}

		switch formatter.Format {
		case output.FormatJSON:
			return formatter.WriteJSON(result)
		default:
			fmt.Fprintln(os.Stderr, "Resume updated successfully")
			return formatter.WriteJSON(result)
		}
	},
}

var resumeDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete master resume",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		if err := apiClient.Delete(ctx, "/resume/master/"+args[0], nil); err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "Resume %s deleted\n", args[0])
		return nil
	},
}

var resumeRenderCmd = &cobra.Command{
	Use:   "render",
	Short: "Render resume to PDF",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		filePath, _ := cmd.Flags().GetString("file")
		theme, _ := cmd.Flags().GetString("theme")

		data, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("reading file: %w", err)
		}

		var resumeData interface{}
		if err := json.Unmarshal(data, &resumeData); err != nil {
			return fmt.Errorf("invalid JSON in %s: %w", filePath, err)
		}

		body := map[string]interface{}{
			"resume": resumeData,
		}
		if theme != "" {
			body["theme"] = theme
		}

		var result struct {
			URL string `json:"url"`
		}
		if err := apiClient.Post(ctx, "/resume/render", body, &result); err != nil {
			return err
		}

		switch formatter.Format {
		case output.FormatJSON:
			return formatter.WriteJSON(result)
		default:
			fmt.Fprintln(os.Stdout, result.URL)
			return nil
		}
	},
}

var resumeThemesCmd = &cobra.Command{
	Use:   "themes",
	Short: "List available themes",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		var result []struct {
			Name             string `json:"name"`
			DisplayName      string `json:"displayName"`
			Description      string `json:"description"`
			SupportsMarkdown bool   `json:"supportsMarkdown"`
		}
		if err := apiClient.Get(ctx, "/resume/themes", &result); err != nil {
			return err
		}

		switch formatter.Format {
		case output.FormatJSON:
			return formatter.WriteJSON(result)
		case output.FormatTable:
			rows := make([][]string, len(result))
			for i, t := range result {
				md := "no"
				if t.SupportsMarkdown {
					md = "yes"
				}
				rows[i] = []string{t.Name, t.DisplayName, t.Description, md}
			}
			return formatter.WriteTable([]string{"NAME", "DISPLAY", "DESCRIPTION", "MARKDOWN"}, rows)
		default:
			for _, t := range result {
				fmt.Fprintf(os.Stdout, "  %s — %s\n", t.Name, t.Description)
			}
			return nil
		}
	},
}

var resumeParseCmd = &cobra.Command{
	Use:   "parse <file>",
	Short: "Parse a resume file into structured data",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return uploadFileAndDecode(args[0], "/resume/parse", "file")
	},
}

var resumeValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate resume data against schema",
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
		if err := apiClient.Post(ctx, "/resume/validate", body, &result); err != nil {
			return err
		}

		return formatter.WriteJSON(result)
	},
}

var resumePreviewCmd = &cobra.Command{
	Use:   "preview",
	Short: "Preview resume as HTML",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		filePath, _ := cmd.Flags().GetString("file")
		theme, _ := cmd.Flags().GetString("theme")

		data, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("reading file: %w", err)
		}

		var resumeData interface{}
		if err := json.Unmarshal(data, &resumeData); err != nil {
			return fmt.Errorf("invalid JSON in %s: %w", filePath, err)
		}

		body := map[string]interface{}{
			"resume": resumeData,
		}
		if theme != "" {
			body["theme"] = theme
		}

		var result struct {
			HTML string `json:"html"`
		}
		if err := apiClient.Post(ctx, "/resume/preview", body, &result); err != nil {
			return err
		}

		switch formatter.Format {
		case output.FormatJSON:
			return formatter.WriteJSON(result)
		default:
			fmt.Fprint(os.Stdout, result.HTML)
			return nil
		}
	},
}

func init() {
	resumeCreateCmd.Flags().String("file", "", "Path to resume JSON file")
	_ = resumeCreateCmd.MarkFlagRequired("file")

	resumeUpdateCmd.Flags().String("file", "", "Path to resume JSON file")
	_ = resumeUpdateCmd.MarkFlagRequired("file")

	resumeRenderCmd.Flags().String("file", "", "Path to resume JSON file")
	resumeRenderCmd.Flags().String("theme", "", "Theme name")
	resumeRenderCmd.Flags().String("dest", "", "Output PDF path")
	_ = resumeRenderCmd.MarkFlagRequired("file")

	resumeValidateCmd.Flags().String("file", "", "Path to resume JSON file")
	_ = resumeValidateCmd.MarkFlagRequired("file")

	resumePreviewCmd.Flags().String("file", "", "Path to resume JSON file")
	resumePreviewCmd.Flags().String("theme", "", "Theme name")
	_ = resumePreviewCmd.MarkFlagRequired("file")

	resumeCmd.AddCommand(resumeListCmd)
	resumeCmd.AddCommand(resumeGetCmd)
	resumeCmd.AddCommand(resumeCreateCmd)
	resumeCmd.AddCommand(resumeUpdateCmd)
	resumeCmd.AddCommand(resumeDeleteCmd)
	resumeCmd.AddCommand(resumeRenderCmd)
	resumeCmd.AddCommand(resumePreviewCmd)
	resumeCmd.AddCommand(resumeParseCmd)
	resumeCmd.AddCommand(resumeValidateCmd)
	resumeCmd.AddCommand(resumeThemesCmd)
	rootCmd.AddCommand(resumeCmd)
}
