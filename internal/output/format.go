package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

// Format represents an output format.
type Format string

const (
	FormatText  Format = "text"
	FormatJSON  Format = "json"
	FormatTable Format = "table"
)

// ParseFormat parses a string into a Format, defaulting to text.
func ParseFormat(s string) Format {
	switch strings.ToLower(s) {
	case "json":
		return FormatJSON
	case "table":
		return FormatTable
	default:
		return FormatText
	}
}

// Formatter handles output rendering for different formats.
type Formatter struct {
	Format Format
	Writer io.Writer
}

// NewFormatter creates a new Formatter.
func NewFormatter(format Format, w io.Writer) *Formatter {
	return &Formatter{Format: format, Writer: w}
}

// WriteJSON writes data as indented JSON.
func (f *Formatter) WriteJSON(data interface{}) error {
	enc := json.NewEncoder(f.Writer)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

// WriteText writes key-value pairs in human-friendly format.
func (f *Formatter) WriteText(title string, fields []KeyValue) error {
	if title != "" {
		if _, err := fmt.Fprintln(f.Writer, title); err != nil {
			return err
		}
	}
	for _, kv := range fields {
		if _, err := fmt.Fprintf(f.Writer, "  %-18s%s\n", kv.Key+":", kv.Value); err != nil {
			return err
		}
	}
	return nil
}

// WriteTable writes tabular data with headers.
func (f *Formatter) WriteTable(headers []string, rows [][]string) error {
	tw := tabwriter.NewWriter(f.Writer, 0, 0, 3, ' ', 0)

	// Header row.
	if _, err := fmt.Fprintln(tw, strings.Join(headers, "\t")); err != nil {
		return err
	}

	// Data rows.
	for _, row := range rows {
		if _, err := fmt.Fprintln(tw, strings.Join(row, "\t")); err != nil {
			return err
		}
	}

	return tw.Flush()
}

// KeyValue is a label-value pair for text output.
type KeyValue struct {
	Key   string
	Value string
}
