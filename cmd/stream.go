package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/llmconveyors/cli/internal/client"
	"github.com/llmconveyors/cli/internal/output"
	"github.com/llmconveyors/cli/internal/sse"
	"github.com/spf13/cobra"
)

var streamCmd = &cobra.Command{
	Use:   "stream <generationId>",
	Short: "Stream raw SSE events for a generation",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		generationID := args[0]
		ctx := context.Background()
		lastEventID, _ := cmd.Flags().GetString("last-event-id")

		return streamGeneration(ctx, generationID, lastEventID)
	},
}

var streamHealthCmd = &cobra.Command{
	Use:   "stream-health",
	Short: "Check SSE stream server health",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		var result map[string]interface{}
		if err := apiClient.Get(ctx, "/stream/health", &result); err != nil {
			return err
		}

		return formatter.WriteJSON(result)
	},
}

func init() {
	streamCmd.Flags().String("last-event-id", "", "Last-Event-ID for reconnection")
	rootCmd.AddCommand(streamCmd)
	rootCmd.AddCommand(streamHealthCmd)
}

// streamGeneration connects to the SSE endpoint and processes events.
// It reconnects with Last-Event-ID on unexpected disconnects.
func streamGeneration(ctx context.Context, generationID, lastEventID string) error {
	retryConfig := client.DefaultRetryConfig()
	var completed bool

	for attempt := 0; attempt <= retryConfig.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := retryConfig.CalculateBackoff(attempt - 1)
			if err := client.Sleep(ctx, delay); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "[RECONNECT] Attempt %d, Last-Event-ID: %s\n", attempt, lastEventID)
		}

		var err error
		lastEventID, completed, err = streamOnce(ctx, generationID, lastEventID)
		if err != nil {
			return err
		}
		if completed {
			return nil
		}

		// Unexpected EOF without complete/error — reconnect.
	}

	return fmt.Errorf("stream disconnected after %d reconnection attempts", retryConfig.MaxRetries)
}

// streamOnce connects to the SSE endpoint once and processes events until
// completion, error, or disconnection.
// Returns (lastEventID, completed, error).
func streamOnce(ctx context.Context, generationID, lastEventID string) (string, bool, error) {
	path := "/stream/generation/" + generationID

	var headers http.Header
	if lastEventID != "" {
		headers = http.Header{}
		headers.Set("Last-Event-ID", lastEventID)
	}

	resp, err := apiClient.GetRaw(ctx, path, headers)
	if err != nil {
		return lastEventID, false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		apiErr := client.ParseErrorResponse(resp)
		return lastEventID, false, apiErr
	}

	return processSSEStream(resp.Body, lastEventID)
}

// processSSEStream reads events from an SSE stream and writes formatted output.
// Returns (lastEventID, completed, error).
func processSSEStream(body io.Reader, lastEventID string) (string, bool, error) {
	parser := sse.NewParser(body)

	for {
		event, err := parser.Next()
		if err == io.EOF {
			// Unexpected disconnect — return for reconnection.
			return parser.LastEventID(), false, nil
		}
		if err != nil {
			return parser.LastEventID(), false, fmt.Errorf("stream error: %w", err)
		}

		// Track Last-Event-ID for reconnection.
		if event.ID != "" {
			lastEventID = event.ID
		}

		switch event.Type {
		case "progress":
			p, err := sse.DecodeProgress(event.Data)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[WARN] bad progress event: %v\n", err)
				continue
			}
			fmt.Fprintf(os.Stderr, "[%d%%] %s: %s\n", p.Percent, p.Step, p.Message)

		case "chunk":
			c, err := sse.DecodeChunk(event.Data)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[WARN] bad chunk event: %v\n", err)
				continue
			}
			fmt.Fprint(os.Stdout, c.Chunk)

		case "log":
			l, err := sse.DecodeLog(event.Data)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[WARN] bad log event: %v\n", err)
				continue
			}
			level := strings.ToUpper(l.Level)
			fmt.Fprintf(os.Stderr, "[%s] %s\n", level, l.Content)

		case "complete":
			c, err := sse.DecodeComplete(event.Data)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[WARN] bad complete event: %v\n", err)
				return parser.LastEventID(), true, nil
			}

			// Per official docs: awaitingInput in complete event signals phased pause.
			if c.AwaitingInput {
				data, _ := json.MarshalIndent(c, "", "  ")
				fmt.Fprintf(os.Stderr, "\nAwaiting input (%s):\n%s\n", c.InteractionType, string(data))
				return parser.LastEventID(), true, ErrAwaitingInput
			}

			if formatter.Format == output.FormatJSON {
				if err := formatter.WriteJSON(c); err != nil {
					return parser.LastEventID(), true, err
				}
			} else {
				fmt.Fprintf(os.Stderr, "\nGeneration complete (job: %s)\n", c.JobID)
				if len(c.Warnings) > 0 {
					for _, w := range c.Warnings {
						fmt.Fprintf(os.Stderr, "[WARN] %s\n", w)
					}
				}
			}
			return parser.LastEventID(), true, nil

		case "error":
			e, err := sse.DecodeError(event.Data)
			if err != nil {
				return parser.LastEventID(), true, fmt.Errorf("stream error (unparseable): %s", string(event.Data))
			}
			return parser.LastEventID(), true, fmt.Errorf("[%s] %s", e.Code, e.Message)

		case "heartbeat":
			// Silently ignore.

		default:
			if cfg.Debug {
				fmt.Fprintf(os.Stderr, "[DEBUG] unknown event type: %s\n", event.Type)
			}
		}
	}
}
