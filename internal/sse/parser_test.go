package sse

import (
	"io"
	"strings"
	"testing"
)

func TestParser_SingleEvent(t *testing.T) {
	stream := "event: progress\ndata: {\"event\":\"progress\",\"data\":{\"step\":\"research\",\"percent\":25,\"message\":\"Researching...\"}}\n\n"

	parser := NewParser(strings.NewReader(stream))
	event, err := parser.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if event.Type != "progress" {
		t.Errorf("Type=%q, want progress", event.Type)
	}

	p, err := DecodeProgress(event.Data)
	if err != nil {
		t.Fatalf("DecodeProgress: %v", err)
	}
	if p.Step != "research" {
		t.Errorf("Step=%q, want research", p.Step)
	}
	if p.Percent != 25 {
		t.Errorf("Percent=%d, want 25", p.Percent)
	}
}

func TestParser_MultipleEvents(t *testing.T) {
	stream := strings.Join([]string{
		"event: progress",
		`data: {"event":"progress","data":{"step":"research","percent":25,"message":"Researching..."}}`,
		"",
		"event: chunk",
		`data: {"event":"chunk","data":{"chunk":"Hello ","index":0}}`,
		"",
		"event: complete",
		`data: {"event":"complete","data":{"jobId":"j1","sessionId":"s1","success":true}}`,
		"",
		"",
	}, "\n")

	parser := NewParser(strings.NewReader(stream))

	e1, err := parser.Next()
	if err != nil {
		t.Fatalf("event 1: %v", err)
	}
	if e1.Type != "progress" {
		t.Errorf("event 1 type=%q, want progress", e1.Type)
	}

	e2, err := parser.Next()
	if err != nil {
		t.Fatalf("event 2: %v", err)
	}
	if e2.Type != "chunk" {
		t.Errorf("event 2 type=%q, want chunk", e2.Type)
	}
	c, _ := DecodeChunk(e2.Data)
	if c.Chunk != "Hello " {
		t.Errorf("chunk=%q, want 'Hello '", c.Chunk)
	}

	e3, err := parser.Next()
	if err != nil {
		t.Fatalf("event 3: %v", err)
	}
	if e3.Type != "complete" {
		t.Errorf("event 3 type=%q, want complete", e3.Type)
	}

	_, err = parser.Next()
	if err != io.EOF {
		t.Errorf("expected EOF, got %v", err)
	}
}

func TestParser_Comments(t *testing.T) {
	stream := ": this is a comment\nevent: progress\ndata: {\"event\":\"progress\",\"data\":{\"step\":\"init\",\"percent\":0,\"message\":\"Starting\"}}\n\n"

	parser := NewParser(strings.NewReader(stream))
	event, err := parser.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event.Type != "progress" {
		t.Errorf("Type=%q, want progress", event.Type)
	}
}

func TestParser_EventID(t *testing.T) {
	stream := "id: 42\nevent: chunk\ndata: {\"event\":\"chunk\",\"data\":{\"chunk\":\"text\",\"index\":0}}\n\n"

	parser := NewParser(strings.NewReader(stream))
	event, err := parser.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event.ID != "42" {
		t.Errorf("ID=%q, want 42", event.ID)
	}
	if parser.LastEventID() != "42" {
		t.Errorf("LastEventID=%q, want 42", parser.LastEventID())
	}
}

func TestParser_EmptyStream(t *testing.T) {
	parser := NewParser(strings.NewReader(""))
	_, err := parser.Next()
	if err != io.EOF {
		t.Errorf("expected EOF, got %v", err)
	}
}

func TestParser_HeartbeatEvent(t *testing.T) {
	stream := "event: heartbeat\ndata: {\"event\":\"heartbeat\",\"data\":{\"timestamp\":\"2026-01-01T00:00:00Z\"}}\n\n"

	parser := NewParser(strings.NewReader(stream))
	event, err := parser.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event.Type != "heartbeat" {
		t.Errorf("Type=%q, want heartbeat", event.Type)
	}
}

func TestParser_ErrorEvent(t *testing.T) {
	stream := "event: error\ndata: {\"event\":\"error\",\"data\":{\"jobId\":\"j1\",\"sessionId\":\"s1\",\"code\":\"AI_PROVIDER_ERROR\",\"message\":\"Upstream failed\"}}\n\n"

	parser := NewParser(strings.NewReader(stream))
	event, err := parser.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	e, err := DecodeError(event.Data)
	if err != nil {
		t.Fatalf("DecodeError: %v", err)
	}
	if e.Code != "AI_PROVIDER_ERROR" {
		t.Errorf("Code=%q, want AI_PROVIDER_ERROR", e.Code)
	}
	if e.Message != "Upstream failed" {
		t.Errorf("Message=%q, want 'Upstream failed'", e.Message)
	}
}

func TestParser_LogEvent(t *testing.T) {
	stream := "event: log\ndata: {\"event\":\"log\",\"data\":{\"content\":\"Processing step 3\",\"level\":\"info\",\"timestamp\":\"2026-01-01T00:00:00Z\"}}\n\n"

	parser := NewParser(strings.NewReader(stream))
	event, err := parser.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	l, err := DecodeLog(event.Data)
	if err != nil {
		t.Fatalf("DecodeLog: %v", err)
	}
	if l.Content != "Processing step 3" {
		t.Errorf("Content=%q, want 'Processing step 3'", l.Content)
	}
	if l.Level != "info" {
		t.Errorf("Level=%q, want info", l.Level)
	}
}

func TestParser_MultipleEmptyLines(t *testing.T) {
	stream := "event: chunk\ndata: {\"event\":\"chunk\",\"data\":{\"chunk\":\"a\",\"index\":0}}\n\n\n\nevent: chunk\ndata: {\"event\":\"chunk\",\"data\":{\"chunk\":\"b\",\"index\":1}}\n\n"

	parser := NewParser(strings.NewReader(stream))

	e1, err := parser.Next()
	if err != nil {
		t.Fatalf("event 1: %v", err)
	}
	c1, _ := DecodeChunk(e1.Data)
	if c1.Chunk != "a" {
		t.Errorf("chunk 1=%q, want a", c1.Chunk)
	}

	e2, err := parser.Next()
	if err != nil {
		t.Fatalf("event 2: %v", err)
	}
	c2, _ := DecodeChunk(e2.Data)
	if c2.Chunk != "b" {
		t.Errorf("chunk 2=%q, want b", c2.Chunk)
	}

	_, err = parser.Next()
	if err != io.EOF {
		t.Errorf("expected EOF, got %v", err)
	}
}

func TestParser_DataWithNoSpace(t *testing.T) {
	// SSE spec: "data:value" (no space) is valid.
	stream := "event: chunk\ndata:{\"event\":\"chunk\",\"data\":{\"chunk\":\"x\",\"index\":0}}\n\n"

	parser := NewParser(strings.NewReader(stream))
	event, err := parser.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c, err := DecodeChunk(event.Data)
	if err != nil {
		t.Fatalf("DecodeChunk: %v", err)
	}
	if c.Chunk != "x" {
		t.Errorf("chunk=%q, want x", c.Chunk)
	}
}

func TestDecodeComplete_WithPhasedFields(t *testing.T) {
	data := []byte(`{"jobId":"j1","sessionId":"s1","success":true,"awaitingInput":true,"interactionType":"contact_selection","completedPhase":"A","warnings":["low confidence"]}`)

	c, err := DecodeComplete(data)
	if err != nil {
		t.Fatalf("DecodeComplete: %v", err)
	}
	if c.JobID != "j1" {
		t.Errorf("JobID=%q, want j1", c.JobID)
	}
	if !c.Success {
		t.Error("Success=false, want true")
	}
	if !c.AwaitingInput {
		t.Error("AwaitingInput=false, want true")
	}
	if c.InteractionType != "contact_selection" {
		t.Errorf("InteractionType=%q, want contact_selection", c.InteractionType)
	}
	if c.CompletedPhase != "A" {
		t.Errorf("CompletedPhase=%q, want A", c.CompletedPhase)
	}
	if len(c.Warnings) != 1 || c.Warnings[0] != "low confidence" {
		t.Errorf("Warnings=%v, want [low confidence]", c.Warnings)
	}
}

func TestDecodeComplete_Minimal(t *testing.T) {
	data := []byte(`{"jobId":"j1","sessionId":"s1","success":true}`)

	c, err := DecodeComplete(data)
	if err != nil {
		t.Fatalf("DecodeComplete: %v", err)
	}
	if c.JobID != "j1" {
		t.Errorf("JobID=%q, want j1", c.JobID)
	}
	if c.AwaitingInput {
		t.Error("AwaitingInput should be false by default")
	}
}
