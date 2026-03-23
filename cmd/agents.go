package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/llmconveyors/cli/internal/output"
	"github.com/spf13/cobra"
)

// Valid agent types.
var validAgentTypes = map[string]bool{
	"job-hunter": true,
	"b2b-sales":  true,
}

func validateAgentType(t string) error {
	if !validAgentTypes[t] {
		return fmt.Errorf("unknown agent type %q (valid: job-hunter, b2b-sales)", t)
	}
	return nil
}

// --- Generate (run) command ---

type generateResponse struct {
	JobID        string `json:"jobId"`
	GenerationID string `json:"generationId"`
	SessionID    string `json:"sessionId"`
	Status       string `json:"status"`
	StreamURL    string `json:"streamUrl"`
}

var runCmd = &cobra.Command{
	Use:   "run <agent-type>",
	Short: "Start an agent generation",
	Long:  "Start a generation for the specified agent type (job-hunter, b2b-sales).",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		agentType := args[0]
		if err := validateAgentType(agentType); err != nil {
			return err
		}

		// Client-side validation of required fields.
		if err := validateRunFlags(cmd, agentType); err != nil {
			return err
		}

		ctx := context.Background()

		// Build request body from flags.
		body := map[string]interface{}{}

		// Common fields.
		setStringIfFlag(cmd, body, "company", "companyName")
		setStringIfFlag(cmd, body, "website", "companyWebsite")
		setStringIfFlag(cmd, body, "session-id", "sessionId")
		setStringIfFlag(cmd, body, "generation-id", "generationId")
		setStringIfFlag(cmd, body, "master-resume-id", "masterResumeId")
		setStringIfFlag(cmd, body, "tier", "tier")
		setStringIfFlag(cmd, body, "model", "model")
		setStringIfFlag(cmd, body, "contact-name", "contactName")
		setStringIfFlag(cmd, body, "contact-email", "contactEmail")
		setStringIfFlag(cmd, body, "theme", "theme")
		setStringIfFlag(cmd, body, "webhook-url", "webhookUrl")

		// Job-hunter specific.
		setStringIfFlag(cmd, body, "title", "jobTitle")
		setStringIfFlag(cmd, body, "jd", "jobDescription")
		setStringIfFlag(cmd, body, "mode", "mode")
		setBoolIfFlag(cmd, body, "auto-select-contacts", "autoSelectContacts")

		// B2B-sales specific.
		setStringIfFlag(cmd, body, "sender-name", "senderName")

		path := fmt.Sprintf("/agents/%s/generate", agentType)

		var result generateResponse
		if err := apiClient.Post(ctx, path, body, &result); err != nil {
			return err
		}

		// Print IDs.
		switch formatter.Format {
		case output.FormatJSON:
			return formatter.WriteJSON(result)
		default:
			if err := formatter.WriteText("Job started successfully", []output.KeyValue{
				{Key: "Job ID", Value: result.JobID},
				{Key: "Session ID", Value: result.SessionID},
				{Key: "Generation ID", Value: result.GenerationID},
				{Key: "Status", Value: result.Status},
				{Key: "Stream URL", Value: result.StreamURL},
			}); err != nil {
				return err
			}
		}

		// Decide wait mode: no-wait > poll > stream (default).
		noWait, _ := cmd.Flags().GetBool("no-wait")
		if noWait {
			return nil
		}

		poll, _ := cmd.Flags().GetBool("poll")
		if poll {
			return pollUntilDone(ctx, agentType, result.JobID)
		}

		// Default: stream.
		return streamGeneration(ctx, result.GenerationID, "")
	},
}

// validateRunFlags checks required fields based on agent type.
func validateRunFlags(cmd *cobra.Command, agentType string) error {
	company, _ := cmd.Flags().GetString("company")
	if company == "" {
		return fmt.Errorf("--company is required")
	}

	switch agentType {
	case "job-hunter":
		title, _ := cmd.Flags().GetString("title")
		jd, _ := cmd.Flags().GetString("jd")
		if title == "" {
			return fmt.Errorf("--title is required for job-hunter")
		}
		if jd == "" {
			return fmt.Errorf("--jd is required for job-hunter")
		}
	case "b2b-sales":
		website, _ := cmd.Flags().GetString("website")
		if website == "" {
			return fmt.Errorf("--website is required for b2b-sales")
		}
	}
	return nil
}

func setStringIfFlag(cmd *cobra.Command, body map[string]interface{}, flag, key string) {
	if cmd.Flags().Changed(flag) {
		val, _ := cmd.Flags().GetString(flag)
		body[key] = val
	}
}

func setBoolIfFlag(cmd *cobra.Command, body map[string]interface{}, flag, key string) {
	if cmd.Flags().Changed(flag) {
		val, _ := cmd.Flags().GetBool(flag)
		body[key] = val
	}
}

// --- Status command ---

type statusResponse struct {
	JobID           string      `json:"jobId"`
	GenerationID    string      `json:"generationId"`
	SessionID       string      `json:"sessionId"`
	AgentType       string      `json:"agentType"`
	Status          string      `json:"status"`
	Progress        int         `json:"progress"`
	CurrentStep     string      `json:"currentStep"`
	Logs            interface{} `json:"logs,omitempty"`
	Artifacts       interface{} `json:"artifacts,omitempty"`
	FailedReason    string      `json:"failedReason,omitempty"`
	InteractionData interface{} `json:"interactionData,omitempty"`
	Result          interface{} `json:"result,omitempty"`
	CreatedAt       string      `json:"createdAt"`
	CompletedAt     string      `json:"completedAt,omitempty"`
}

var statusCmd = &cobra.Command{
	Use:   "status <jobId>",
	Short: "Check generation status",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		jobID := args[0]
		agentType, _ := cmd.Flags().GetString("agent")
		if err := validateAgentType(agentType); err != nil {
			return err
		}

		ctx := context.Background()
		watch, _ := cmd.Flags().GetBool("watch")
		include, _ := cmd.Flags().GetString("include")

		if watch {
			return pollUntilDone(ctx, agentType, jobID)
		}

		path := fmt.Sprintf("/agents/%s/status/%s", agentType, jobID)
		if include != "" {
			path += "?include=" + include
		}

		var result statusResponse
		if err := apiClient.Get(ctx, path, &result); err != nil {
			return err
		}

		switch formatter.Format {
		case output.FormatJSON:
			return formatter.WriteJSON(result)
		default:
			fields := []output.KeyValue{
				{Key: "Job ID", Value: result.JobID},
				{Key: "Agent", Value: result.AgentType},
				{Key: "Status", Value: result.Status},
				{Key: "Progress", Value: fmt.Sprintf("%d%%", result.Progress)},
				{Key: "Current Step", Value: result.CurrentStep},
				{Key: "Created", Value: result.CreatedAt},
			}
			if result.CompletedAt != "" {
				fields = append(fields, output.KeyValue{Key: "Completed", Value: result.CompletedAt})
			}
			if result.FailedReason != "" {
				fields = append(fields, output.KeyValue{Key: "Failed Reason", Value: result.FailedReason})
			}
			return formatter.WriteText("Generation Status", fields)
		}
	},
}

// pollUntilDone polls the status endpoint until a terminal state.
func pollUntilDone(ctx context.Context, agentType, jobID string) error {
	path := fmt.Sprintf("/agents/%s/status/%s", agentType, jobID)

	for {
		var result statusResponse
		if err := apiClient.Get(ctx, path, &result); err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "[%d%%] %s: %s\n", result.Progress, result.CurrentStep, result.Status)

		switch result.Status {
		case "completed":
			if formatter.Format == output.FormatJSON {
				return formatter.WriteJSON(result)
			}
			fmt.Fprintf(os.Stderr, "Generation completed (job: %s)\n", result.JobID)
			return nil
		case "failed":
			return fmt.Errorf("generation failed: %s", result.FailedReason)
		case "awaiting_input":
			data, _ := json.MarshalIndent(result.InteractionData, "", "  ")
			fmt.Fprintf(os.Stderr, "\nAwaiting input:\n%s\n", string(data))
			return ErrAwaitingInput
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

// --- Interact command ---

var interactCmd = &cobra.Command{
	Use:   "interact",
	Short: "Resume a phased generation with interaction data",
	RunE: func(cmd *cobra.Command, args []string) error {
		agentType, _ := cmd.Flags().GetString("agent")
		if err := validateAgentType(agentType); err != nil {
			return err
		}

		generationID, _ := cmd.Flags().GetString("generation-id")
		sessionID, _ := cmd.Flags().GetString("session-id")
		interactionType, _ := cmd.Flags().GetString("type")
		dataStr, _ := cmd.Flags().GetString("data")

		if generationID == "" || sessionID == "" || interactionType == "" {
			return fmt.Errorf("--generation-id, --session-id, and --type are required")
		}

		var interactionData interface{}
		if dataStr != "" {
			if err := json.Unmarshal([]byte(dataStr), &interactionData); err != nil {
				return fmt.Errorf("invalid --data JSON: %w", err)
			}
		}

		body := map[string]interface{}{
			"generationId":    generationID,
			"sessionId":       sessionID,
			"interactionType": interactionType,
			"interactionData": interactionData,
		}

		path := fmt.Sprintf("/agents/%s/interact", agentType)
		ctx := context.Background()

		var result struct {
			JobID        string `json:"jobId,omitempty"`
			GenerationID string `json:"generationId,omitempty"`
			StreamURL    string `json:"streamUrl,omitempty"`
		}
		if err := apiClient.Post(ctx, path, body, &result); err != nil {
			return err
		}

		switch formatter.Format {
		case output.FormatJSON:
			return formatter.WriteJSON(result)
		default:
			fields := []output.KeyValue{}
			if result.JobID != "" {
				fields = append(fields, output.KeyValue{Key: "Job ID", Value: result.JobID})
			}
			if result.StreamURL != "" {
				fields = append(fields, output.KeyValue{Key: "Stream URL", Value: result.StreamURL})
			}
			if err := formatter.WriteText("Interaction submitted", fields); err != nil {
				return err
			}
		}

		// Auto-stream Phase B if --stream flag is set and we have a generation ID.
		shouldStream, _ := cmd.Flags().GetBool("stream")
		if shouldStream && result.GenerationID != "" {
			return streamGeneration(ctx, result.GenerationID, "")
		}

		return nil
	},
}

// --- Manifest command ---

var manifestCmd = &cobra.Command{
	Use:   "manifest <agent-type>",
	Short: "Show agent capabilities",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		agentType := args[0]
		if err := validateAgentType(agentType); err != nil {
			return err
		}

		ctx := context.Background()
		path := fmt.Sprintf("/agents/%s/manifest", agentType)

		// Manifest is freeform — decode as generic map.
		var result map[string]interface{}
		if err := apiClient.Get(ctx, path, &result); err != nil {
			return err
		}

		return formatter.WriteJSON(result)
	},
}

func init() {
	// Run command flags.
	runCmd.Flags().String("company", "", "Company name (required)")
	runCmd.Flags().String("website", "", "Company website URL")
	runCmd.Flags().String("title", "", "Job title (job-hunter, required)")
	runCmd.Flags().String("jd", "", "Job description text (job-hunter, required)")
	runCmd.Flags().String("session-id", "", "Existing session ID")
	runCmd.Flags().String("generation-id", "", "Existing generation ID")
	runCmd.Flags().String("master-resume-id", "", "Master resume ID")
	runCmd.Flags().String("tier", "", "Tier: free or byo")
	runCmd.Flags().String("model", "", "Model: flash or pro")
	runCmd.Flags().String("contact-name", "", "Contact name")
	runCmd.Flags().String("contact-email", "", "Contact email")
	runCmd.Flags().String("theme", "", "Resume theme")
	runCmd.Flags().String("mode", "", "Mode: standard or cold_outreach (job-hunter)")
	runCmd.Flags().String("sender-name", "", "Sender name (b2b-sales)")
	runCmd.Flags().String("webhook-url", "", "Webhook URL for async notifications")
	runCmd.Flags().Bool("auto-select-contacts", false, "Auto-select contacts (skip interaction gate)")
	runCmd.Flags().Bool("poll", false, "Poll status instead of streaming")
	runCmd.Flags().Bool("no-wait", false, "Print IDs and exit immediately")

	// Status command flags.
	statusCmd.Flags().String("agent", "", "Agent type (required)")
	statusCmd.Flags().Bool("watch", false, "Poll until terminal state")
	statusCmd.Flags().String("include", "", "Include extra data: logs,artifacts")
	_ = statusCmd.MarkFlagRequired("agent")

	// Interact command flags.
	interactCmd.Flags().String("agent", "", "Agent type (required)")
	interactCmd.Flags().String("generation-id", "", "Generation ID (required)")
	interactCmd.Flags().String("session-id", "", "Session ID (required)")
	interactCmd.Flags().String("type", "", "Interaction type (required)")
	interactCmd.Flags().String("data", "", "Interaction data as JSON")
	interactCmd.Flags().Bool("stream", false, "Auto-stream Phase B after interaction")
	_ = interactCmd.MarkFlagRequired("agent")

	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(interactCmd)
	rootCmd.AddCommand(manifestCmd)
}
