package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"

	"github.com/ebenezer-isaac/llmconveyors-go-package/internal/output"
	"github.com/spf13/cobra"
)

var uploadCmd = &cobra.Command{
	Use:   "upload",
	Short: "Upload and parse documents",
}

var uploadResumeCmd = &cobra.Command{
	Use:   "resume <file>",
	Short: "Upload and parse a resume",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return uploadFile(args[0], "/upload/resume", "file")
	},
}

var uploadJobCmd = &cobra.Command{
	Use:   "job <file>",
	Short: "Upload and parse a job description",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return uploadFile(args[0], "/upload/job", "file")
	},
}

var uploadJobTextCmd = &cobra.Command{
	Use:   "job-text",
	Short: "Parse job description from text",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		body := map[string]interface{}{}

		filePath, _ := cmd.Flags().GetString("file")
		text, _ := cmd.Flags().GetString("text")
		source, _ := cmd.Flags().GetString("source")

		if filePath != "" {
			data, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("reading file: %w", err)
			}
			body["text"] = string(data)
		} else if text != "" {
			body["text"] = text
		} else {
			return fmt.Errorf("--file or --text is required")
		}

		if source != "" {
			body["source"] = source
		}

		var result map[string]interface{}
		if err := apiClient.Post(ctx, "/upload/job-text", body, &result); err != nil {
			return err
		}

		return formatter.WriteJSON(result)
	},
}

func uploadFile(filePath, endpoint, fieldName string) error {
	ctx := context.Background()

	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile(fieldName, filepath.Base(filePath))
	if err != nil {
		return fmt.Errorf("creating form file: %w", err)
	}

	if _, err := io.Copy(part, f); err != nil {
		return fmt.Errorf("writing file to form: %w", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("closing multipart writer: %w", err)
	}

	var result map[string]interface{}
	if err := apiClient.PostMultipart(ctx, endpoint, writer.FormDataContentType(), &buf, &result); err != nil {
		return err
	}

	switch formatter.Format {
	case output.FormatJSON:
		return formatter.WriteJSON(result)
	default:
		fmt.Fprintf(os.Stderr, "Upload successful: %s\n", filePath)
		return formatter.WriteJSON(result)
	}
}

// uploadFileAndDecode is like uploadFile but returns JSON result (used by resume parse).
func uploadFileAndDecode(filePath, endpoint, fieldName string) error {
	return uploadFile(filePath, endpoint, fieldName)
}

func init() {
	uploadJobTextCmd.Flags().String("file", "", "Path to text file")
	uploadJobTextCmd.Flags().String("text", "", "Job description text")
	uploadJobTextCmd.Flags().String("source", "", "Source URL (optional)")

	uploadCmd.AddCommand(uploadResumeCmd)
	uploadCmd.AddCommand(uploadJobCmd)
	uploadCmd.AddCommand(uploadJobTextCmd)
	rootCmd.AddCommand(uploadCmd)
}
