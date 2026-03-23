package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseFormat(t *testing.T) {
	tests := []struct {
		input string
		want  Format
	}{
		{"json", FormatJSON},
		{"JSON", FormatJSON},
		{"table", FormatTable},
		{"TABLE", FormatTable},
		{"text", FormatText},
		{"TEXT", FormatText},
		{"", FormatText},
		{"unknown", FormatText},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParseFormat(tt.input)
			if got != tt.want {
				t.Errorf("ParseFormat(%q)=%q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFormatter_WriteJSON(t *testing.T) {
	var buf bytes.Buffer
	f := NewFormatter(FormatJSON, &buf)

	data := map[string]string{"name": "test", "status": "ok"}
	if err := f.WriteJSON(data); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, `"name": "test"`) {
		t.Errorf("output missing name: %s", out)
	}
	if !strings.Contains(out, `"status": "ok"`) {
		t.Errorf("output missing status: %s", out)
	}
}

func TestFormatter_WriteText(t *testing.T) {
	var buf bytes.Buffer
	f := NewFormatter(FormatText, &buf)

	err := f.WriteText("Test Output", []KeyValue{
		{Key: "Name", Value: "test"},
		{Key: "Status", Value: "ok"},
	})
	if err != nil {
		t.Fatalf("WriteText: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Test Output") {
		t.Errorf("missing title: %s", out)
	}
	if !strings.Contains(out, "Name:") {
		t.Errorf("missing Name key: %s", out)
	}
	if !strings.Contains(out, "test") {
		t.Errorf("missing test value: %s", out)
	}
}

func TestFormatter_WriteText_NoTitle(t *testing.T) {
	var buf bytes.Buffer
	f := NewFormatter(FormatText, &buf)

	err := f.WriteText("", []KeyValue{
		{Key: "ID", Value: "123"},
	})
	if err != nil {
		t.Fatalf("WriteText: %v", err)
	}

	out := buf.String()
	if strings.HasPrefix(out, "\n") {
		t.Error("should not start with newline when title is empty")
	}
	if !strings.Contains(out, "ID:") {
		t.Errorf("missing ID key: %s", out)
	}
}

func TestFormatter_WriteTable(t *testing.T) {
	var buf bytes.Buffer
	f := NewFormatter(FormatTable, &buf)

	err := f.WriteTable(
		[]string{"ID", "NAME", "STATUS"},
		[][]string{
			{"1", "alpha", "active"},
			{"2", "beta", "inactive"},
		},
	)
	if err != nil {
		t.Fatalf("WriteTable: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "ID") {
		t.Errorf("missing header: %s", out)
	}
	if !strings.Contains(out, "alpha") {
		t.Errorf("missing row data: %s", out)
	}
	if !strings.Contains(out, "beta") {
		t.Errorf("missing row data: %s", out)
	}
}

func TestFormatter_WriteTable_Empty(t *testing.T) {
	var buf bytes.Buffer
	f := NewFormatter(FormatTable, &buf)

	err := f.WriteTable([]string{"ID"}, [][]string{})
	if err != nil {
		t.Fatalf("WriteTable: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "ID") {
		t.Errorf("missing header even for empty table: %s", out)
	}
}
