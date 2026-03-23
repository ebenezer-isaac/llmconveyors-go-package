package client

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestAPIError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  APIError
		want string
	}{
		{
			name: "basic",
			err:  APIError{Code: "NOT_FOUND", Message: "Job not found"},
			want: "[NOT_FOUND] Job not found",
		},
		{
			name: "with hint",
			err:  APIError{Code: "RATE_LIMITED", Message: "Slow down", Hint: "Wait 30 seconds"},
			want: "[RATE_LIMITED] Slow down (hint: Wait 30 seconds)",
		},
		{
			name: "empty hint",
			err:  APIError{Code: "UNAUTHORIZED", Message: "Invalid key"},
			want: "[UNAUTHORIZED] Invalid key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAPIError_IsRetryable(t *testing.T) {
	tests := []struct {
		code   string
		status int
		want   bool
	}{
		{CodeRateLimited, 429, true},
		{CodeConcurrentGenLimit, 429, true},
		{CodeGenerationTimeout, 504, true},
		{CodeServerRestarting, 0, true},
		{CodeStreamError, 0, true},
		{CodeAIProviderError, 200, false}, // NOT retryable by code per official docs.
		{CodeAIProviderError, 502, true},  // Retryable by HTTP status.
		{CodeNotFound, 404, false},
		{CodeUnauthorized, 401, false},
		{CodeValidationError, 400, false},
		{CodeInternalError, 500, false},
		{"", 502, true},
		{"", 503, true},
		{"", 504, true},
		{"", 200, false},
	}

	for _, tt := range tests {
		t.Run(tt.code+"-"+http.StatusText(tt.status), func(t *testing.T) {
			e := &APIError{Code: tt.code, HTTPStatus: tt.status}
			got := e.IsRetryable()
			if got != tt.want {
				t.Errorf("IsRetryable()=%v for code=%q status=%d, want %v", got, tt.code, tt.status, tt.want)
			}
		})
	}
}

func TestParseErrorResponse_ValidJSON(t *testing.T) {
	body := `{"success":false,"error":{"code":"VALIDATION_ERROR","message":"Invalid input","hint":"Check the docs","details":{"fieldErrors":{"name":["required"]}}},"requestId":"req-123","path":"/api/v1/test"}`

	resp := &http.Response{
		StatusCode: 400,
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	apiErr := ParseErrorResponse(resp)

	if apiErr.Code != "VALIDATION_ERROR" {
		t.Errorf("Code=%q, want VALIDATION_ERROR", apiErr.Code)
	}
	if apiErr.Message != "Invalid input" {
		t.Errorf("Message=%q, want 'Invalid input'", apiErr.Message)
	}
	if apiErr.Hint != "Check the docs" {
		t.Errorf("Hint=%q, want 'Check the docs'", apiErr.Hint)
	}
	if apiErr.RequestID != "req-123" {
		t.Errorf("RequestID=%q, want req-123", apiErr.RequestID)
	}
	if apiErr.Details == nil {
		t.Fatal("Details is nil")
	}
	if errs, ok := apiErr.Details.FieldErrors["name"]; !ok || len(errs) == 0 {
		t.Error("expected field error for 'name'")
	}
	if apiErr.HTTPStatus != 400 {
		t.Errorf("HTTPStatus=%d, want 400", apiErr.HTTPStatus)
	}
}

func TestParseErrorResponse_MalformedJSON(t *testing.T) {
	resp := &http.Response{
		StatusCode: 500,
		Body:       io.NopCloser(strings.NewReader("not json")),
	}

	apiErr := ParseErrorResponse(resp)
	if apiErr.Code != CodeMalformedResponse {
		t.Errorf("Code=%q, want %q", apiErr.Code, CodeMalformedResponse)
	}
	if apiErr.HTTPStatus != 500 {
		t.Errorf("HTTPStatus=%d, want 500", apiErr.HTTPStatus)
	}
}

func TestParseErrorResponse_EmptyBody(t *testing.T) {
	resp := &http.Response{
		StatusCode: 502,
		Body:       io.NopCloser(strings.NewReader("")),
	}

	apiErr := ParseErrorResponse(resp)
	if apiErr.Code != CodeMalformedResponse {
		t.Errorf("Code=%q, want %q", apiErr.Code, CodeMalformedResponse)
	}
}

func TestIsRetryableCode(t *testing.T) {
	retryable := []string{
		CodeRateLimited, CodeConcurrentGenLimit,
		CodeGenerationTimeout, CodeServerRestarting, CodeStreamError,
	}
	for _, code := range retryable {
		if !IsRetryableCode(code) {
			t.Errorf("IsRetryableCode(%q)=false, want true", code)
		}
	}

	notRetryable := []string{
		CodeNotFound, CodeUnauthorized, CodeForbidden, CodeValidationError,
		CodeInternalError, CodeConflict, CodeAIProviderError, "",
	}
	for _, code := range notRetryable {
		if IsRetryableCode(code) {
			t.Errorf("IsRetryableCode(%q)=true, want false", code)
		}
	}
}

func TestIsRetryableStatus(t *testing.T) {
	retryable := []int{502, 503, 504}
	for _, s := range retryable {
		if !IsRetryableStatus(s) {
			t.Errorf("IsRetryableStatus(%d)=false, want true", s)
		}
	}

	notRetryable := []int{200, 201, 400, 401, 403, 404, 429, 500}
	for _, s := range notRetryable {
		if IsRetryableStatus(s) {
			t.Errorf("IsRetryableStatus(%d)=true, want false", s)
		}
	}
}
