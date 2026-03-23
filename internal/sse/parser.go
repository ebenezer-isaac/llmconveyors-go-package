package sse

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Event represents a single SSE event.
type Event struct {
	Type string          // "progress", "chunk", "log", "complete", "error", "heartbeat"
	Data json.RawMessage // Raw JSON payload
	ID   string          // Last-Event-ID
}

// ProgressData is the payload for "progress" events.
type ProgressData struct {
	JobID     string `json:"jobId"`
	SessionID string `json:"sessionId"`
	Step      string `json:"step"`
	Percent   int    `json:"percent"`
	Message   string `json:"message"`
}

// ChunkData is the payload for "chunk" events.
type ChunkData struct {
	JobID     string `json:"jobId"`
	SessionID string `json:"sessionId"`
	Chunk     string `json:"chunk"`
	Index     int    `json:"index"`
}

// LogData is the payload for "log" events.
type LogData struct {
	MessageID    string `json:"messageId"`
	GenerationID string `json:"generationId"`
	SessionID    string `json:"sessionId"`
	Content      string `json:"content"`
	Level        string `json:"level"`
	Timestamp    string `json:"timestamp"`
}

// CompleteData is the payload for "complete" events.
// Fields match the official SSE streaming docs.
type CompleteData struct {
	JobID               string      `json:"jobId"`
	SessionID           string      `json:"sessionId"`
	GenerationID        string      `json:"generationId,omitempty"`
	Success             bool        `json:"success"`
	Artifacts           interface{} `json:"artifacts"`
	MergedArtifactState interface{} `json:"mergedArtifactState,omitempty"`
	Error               string      `json:"error,omitempty"`
	Warnings            []string    `json:"warnings,omitempty"`
	AwaitingInput       bool        `json:"awaitingInput,omitempty"`
	InteractionType     string      `json:"interactionType,omitempty"`
	CompletedPhase      string      `json:"completedPhase,omitempty"`
	InteractionData     interface{} `json:"interactionData,omitempty"`
	PersistenceWarning  bool        `json:"persistenceWarning,omitempty"`
}

// ErrorData is the payload for "error" events.
type ErrorData struct {
	JobID     string `json:"jobId"`
	SessionID string `json:"sessionId"`
	Code      string `json:"code"`
	Message   string `json:"message"`
}

// Parser reads an SSE stream and emits events.
type Parser struct {
	reader *bufio.Scanner
	lastID string
}

// NewParser creates a new SSE parser from a reader.
func NewParser(r io.Reader) *Parser {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // 1MB max line
	return &Parser{reader: scanner}
}

// LastEventID returns the last event ID seen.
func (p *Parser) LastEventID() string {
	return p.lastID
}

// Next reads the next SSE event from the stream.
// Returns io.EOF when the stream ends.
func (p *Parser) Next() (Event, error) {
	var eventType string
	var dataLines []string
	var id string

	for p.reader.Scan() {
		line := p.reader.Text()

		// Empty line = dispatch event.
		if line == "" {
			if len(dataLines) == 0 {
				continue // No data accumulated, keep reading.
			}

			data := strings.Join(dataLines, "\n")

			// Unwrap the outer envelope: {"event":"...","data":{...}}
			var envelope struct {
				Event string          `json:"event"`
				Data  json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal([]byte(data), &envelope); err == nil && len(envelope.Data) > 0 {
				if eventType == "" {
					eventType = envelope.Event
				}
				return Event{
					Type: eventType,
					Data: envelope.Data,
					ID:   id,
				}, nil
			}

			// Fallback: raw data without envelope.
			return Event{
				Type: eventType,
				Data: json.RawMessage(data),
				ID:   id,
			}, nil
		}

		// Comment line (starts with ':').
		if strings.HasPrefix(line, ":") {
			continue
		}

		// Parse field.
		if strings.HasPrefix(line, "event:") {
			eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		} else if strings.HasPrefix(line, "id:") {
			id = strings.TrimSpace(strings.TrimPrefix(line, "id:"))
			p.lastID = id
		} else if strings.HasPrefix(line, "retry:") {
			// Retry field — ignored for now.
		}
	}

	if err := p.reader.Err(); err != nil {
		return Event{}, fmt.Errorf("reading SSE stream: %w", err)
	}

	return Event{}, io.EOF
}

// DecodeProgress decodes a progress event payload.
func DecodeProgress(data json.RawMessage) (ProgressData, error) {
	var p ProgressData
	err := json.Unmarshal(data, &p)
	return p, err
}

// DecodeChunk decodes a chunk event payload.
func DecodeChunk(data json.RawMessage) (ChunkData, error) {
	var c ChunkData
	err := json.Unmarshal(data, &c)
	return c, err
}

// DecodeLog decodes a log event payload.
func DecodeLog(data json.RawMessage) (LogData, error) {
	var l LogData
	err := json.Unmarshal(data, &l)
	return l, err
}

// DecodeComplete decodes a complete event payload.
func DecodeComplete(data json.RawMessage) (CompleteData, error) {
	var c CompleteData
	err := json.Unmarshal(data, &c)
	return c, err
}

// DecodeError decodes an error event payload.
func DecodeError(data json.RawMessage) (ErrorData, error) {
	var e ErrorData
	err := json.Unmarshal(data, &e)
	return e, err
}
