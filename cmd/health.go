package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/llmconveyors/cli/internal/output"
	"github.com/spf13/cobra"
)

type healthResponse struct {
	Status    string       `json:"status"`
	Timestamp string       `json:"timestamp"`
	Uptime    float64      `json:"uptime"`
	Version   string       `json:"version"`
	Checks    healthChecks `json:"checks"`
	Memory    interface{}  `json:"memory,omitempty"`
}

type healthChecks struct {
	Mongo string `json:"mongo"`
	Redis string `json:"redis"`
}

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check API health status",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		// Health endpoint requires no auth — make a direct HTTP call.
		url := cfg.BaseURL + "/health"
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return fmt.Errorf("creating request: %w", err)
		}
		req.Header.Set("Accept", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("health check failed: %w", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("reading response: %w", err)
		}

		var envelope struct {
			Success bool           `json:"success"`
			Data    healthResponse `json:"data"`
		}
		if err := json.Unmarshal(body, &envelope); err != nil {
			return fmt.Errorf("invalid response: %w", err)
		}

		result := envelope.Data

		switch formatter.Format {
		case output.FormatJSON:
			return formatter.WriteJSON(result)
		case output.FormatTable:
			return formatter.WriteTable(
				[]string{"FIELD", "VALUE"},
				[][]string{
					{"Status", result.Status},
					{"Version", result.Version},
					{"Mongo", result.Checks.Mongo},
					{"Redis", result.Checks.Redis},
					{"Timestamp", result.Timestamp},
				},
			)
		default:
			return formatter.WriteText("API Health", []output.KeyValue{
				{Key: "Status", Value: result.Status},
				{Key: "Version", Value: result.Version},
				{Key: "Mongo", Value: result.Checks.Mongo},
				{Key: "Redis", Value: result.Checks.Redis},
				{Key: "Timestamp", Value: result.Timestamp},
			})
		}
	},
}

func init() {
	rootCmd.AddCommand(healthCmd)
}
