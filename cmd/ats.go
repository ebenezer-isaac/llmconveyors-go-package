package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/ebenezer-isaac/llmconveyors-go-package/internal/output"
	"github.com/spf13/cobra"
)

var atsCmd = &cobra.Command{
	Use:   "ats",
	Short: "ATS scoring",
}

var atsScoreCmd = &cobra.Command{
	Use:   "score",
	Short: "Score resume against job description",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		body := map[string]interface{}{}

		// Resume source: file, ID, or text.
		resumeFile, _ := cmd.Flags().GetString("resume-file")
		resumeID, _ := cmd.Flags().GetString("resume-id")
		if resumeFile != "" {
			data, err := os.ReadFile(resumeFile)
			if err != nil {
				return fmt.Errorf("reading resume file: %w", err)
			}
			body["resumeText"] = string(data)
		} else if resumeID != "" {
			body["masterResumeId"] = resumeID
		} else {
			return fmt.Errorf("--resume-file or --resume-id is required")
		}

		// JD source: file or text.
		jdFile, _ := cmd.Flags().GetString("jd-file")
		jd, _ := cmd.Flags().GetString("jd")
		if jdFile != "" {
			data, err := os.ReadFile(jdFile)
			if err != nil {
				return fmt.Errorf("reading JD file: %w", err)
			}
			body["jobDescription"] = string(data)
		} else if jd != "" {
			body["jobDescription"] = jd
		} else {
			return fmt.Errorf("--jd-file or --jd is required")
		}

		var result map[string]interface{}
		if err := apiClient.Post(ctx, "/ats/score", body, &result); err != nil {
			return err
		}

		switch formatter.Format {
		case output.FormatJSON:
			return formatter.WriteJSON(result)
		default:
			if overall, ok := result["overall"]; ok {
				fmt.Fprintf(os.Stdout, "Overall ATS Score: %v\n", overall)
			}
			return formatter.WriteJSON(result)
		}
	},
}

func init() {
	atsScoreCmd.Flags().String("resume-file", "", "Path to resume text file")
	atsScoreCmd.Flags().String("resume-id", "", "Master resume ID")
	atsScoreCmd.Flags().String("jd-file", "", "Path to job description text file")
	atsScoreCmd.Flags().String("jd", "", "Job description text")

	atsCmd.AddCommand(atsScoreCmd)
	rootCmd.AddCommand(atsCmd)
}
