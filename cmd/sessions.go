package cmd

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/llmconveyors/cli/internal/output"
	"github.com/spf13/cobra"
)

type session struct {
	ID        string `json:"id"`
	AgentType string `json:"agentType"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

type sessionListResponse struct {
	Sessions []session `json:"sessions"`
	Total    int       `json:"total"`
	Page     int       `json:"page"`
	Limit    int       `json:"limit"`
}

var sessionsCmd = &cobra.Command{
	Use:   "sessions",
	Short: "Manage sessions",
}

var sessionsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List sessions",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		page, _ := cmd.Flags().GetInt("page")
		limit, _ := cmd.Flags().GetInt("limit")
		agent, _ := cmd.Flags().GetString("agent")

		path := fmt.Sprintf("/sessions?page=%d&limit=%d", page, limit)
		if agent != "" {
			path += "&agentType=" + agent
		}

		var result sessionListResponse
		if err := apiClient.Get(ctx, path, &result); err != nil {
			return err
		}

		switch formatter.Format {
		case output.FormatJSON:
			return formatter.WriteJSON(result)
		case output.FormatTable:
			rows := make([][]string, len(result.Sessions))
			for i, s := range result.Sessions {
				rows[i] = []string{s.ID, s.AgentType, s.Status, s.CreatedAt}
			}
			return formatter.WriteTable([]string{"ID", "AGENT", "STATUS", "CREATED"}, rows)
		default:
			for _, s := range result.Sessions {
				formatter.WriteText("", []output.KeyValue{
					{Key: "ID", Value: s.ID},
					{Key: "Agent", Value: s.AgentType},
					{Key: "Status", Value: s.Status},
					{Key: "Created", Value: s.CreatedAt},
				})
				fmt.Fprintln(os.Stdout)
			}
			fmt.Fprintf(os.Stderr, "Total: %d (page %d/%d)\n", result.Total, result.Page, (result.Total+result.Limit-1)/max(result.Limit, 1))
			return nil
		}
	},
}

var sessionsGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get session details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		var result map[string]interface{}
		if err := apiClient.Get(ctx, "/sessions/"+args[0], &result); err != nil {
			return err
		}

		return formatter.WriteJSON(result)
	},
}

var sessionsDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		if err := apiClient.Delete(ctx, "/sessions/"+args[0], nil); err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "Session %s deleted\n", args[0])
		return nil
	},
}

var sessionsHydrateCmd = &cobra.Command{
	Use:   "hydrate <id>",
	Short: "Get session with full artifacts",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		var result map[string]interface{}
		if err := apiClient.Get(ctx, "/sessions/"+args[0]+"/hydrate", &result); err != nil {
			return err
		}

		return formatter.WriteJSON(result)
	},
}

var sessionsDownloadCmd = &cobra.Command{
	Use:   "download <id>",
	Short: "Download an artifact file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		key, _ := cmd.Flags().GetString("key")
		outputPath, _ := cmd.Flags().GetString("dest")

		if key == "" {
			return fmt.Errorf("--key is required")
		}

		path := fmt.Sprintf("/sessions/%s/download?key=%s", args[0], key)
		resp, err := apiClient.GetRaw(ctx, path)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		var w io.Writer
		if outputPath != "" {
			f, err := os.Create(outputPath)
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

		if outputPath != "" {
			fmt.Fprintf(os.Stderr, "Downloaded %d bytes to %s\n", n, outputPath)
		}

		return nil
	},
}

var sessionsInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Get session initialization data",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		var result map[string]interface{}
		if err := apiClient.Get(ctx, "/sessions/init", &result); err != nil {
			return err
		}

		return formatter.WriteJSON(result)
	},
}

var sessionsLogCmd = &cobra.Command{
	Use:   "log <id>",
	Short: "Append a chat log entry to a session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		role, _ := cmd.Flags().GetString("role")
		content, _ := cmd.Flags().GetString("content")

		if role == "" {
			return fmt.Errorf("--role is required (user, assistant, system, tool, status)")
		}

		body := map[string]interface{}{
			"role": role,
		}
		if content != "" {
			body["content"] = content
		}

		path := fmt.Sprintf("/sessions/%s/log", args[0])
		var result map[string]interface{}
		if err := apiClient.Post(ctx, path, body, &result); err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "Log entry appended to session %s\n", args[0])
		return nil
	},
}

var sessionsGenLogInitCmd = &cobra.Command{
	Use:   "gen-log-init <sessionId> <generationId>",
	Short: "Initialize a generation log entry",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		path := fmt.Sprintf("/sessions/%s/generation-logs/%s/init", args[0], args[1])
		var result map[string]interface{}
		if err := apiClient.Post(ctx, path, nil, &result); err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "Generation log initialized for session %s, generation %s\n", args[0], args[1])
		return nil
	},
}

func init() {
	sessionsListCmd.Flags().Int("page", 1, "Page number")
	sessionsListCmd.Flags().Int("limit", 20, "Items per page")
	sessionsListCmd.Flags().String("agent", "", "Filter by agent type")

	sessionsDownloadCmd.Flags().String("key", "", "Artifact key to download")
	sessionsDownloadCmd.Flags().String("dest", "", "Output file path")

	sessionsLogCmd.Flags().String("role", "", "Log role: user, assistant, system, tool, status")
	sessionsLogCmd.Flags().String("content", "", "Log content text")

	sessionsCmd.AddCommand(sessionsInitCmd)
	sessionsCmd.AddCommand(sessionsListCmd)
	sessionsCmd.AddCommand(sessionsGetCmd)
	sessionsCmd.AddCommand(sessionsDeleteCmd)
	sessionsCmd.AddCommand(sessionsHydrateCmd)
	sessionsCmd.AddCommand(sessionsDownloadCmd)
	sessionsCmd.AddCommand(sessionsLogCmd)
	sessionsCmd.AddCommand(sessionsGenLogInitCmd)
	rootCmd.AddCommand(sessionsCmd)
}
