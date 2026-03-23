package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// errorDetail contains the structured error from the API.
type errorDetail struct {
	Code    string        `json:"code"`
	Message string        `json:"message"`
	Hint    string        `json:"hint,omitempty"`
	Details *ErrorDetails `json:"details,omitempty"`
}

// ErrorDetails contains field-level validation errors.
type ErrorDetails struct {
	FieldErrors map[string][]string `json:"fieldErrors,omitempty"`
}

// errorResponse is the raw JSON error envelope from the API.
type errorResponse struct {
	Success   bool        `json:"success"`
	Error     errorDetail `json:"error"`
	RequestID string      `json:"requestId,omitempty"`
	Timestamp string      `json:"timestamp,omitempty"`
	Path      string      `json:"path,omitempty"`
}

// APIError represents an error returned by the LLM Conveyors API.
type APIError struct {
	HTTPStatus int
	Code       string
	Message    string
	Hint       string
	Details    *ErrorDetails
	RequestID  string
	Path       string
}

func (e *APIError) Error() string {
	msg := fmt.Sprintf("[%s] %s", e.Code, e.Message)
	if e.Hint != "" {
		msg += " (hint: " + e.Hint + ")"
	}
	return msg
}

// IsRetryable returns true if this error should trigger a retry.
func (e *APIError) IsRetryable() bool {
	return IsRetryableCode(e.Code) || IsRetryableStatus(e.HTTPStatus)
}

// Well-known API error codes.
const (
	CodeValidationError       = "VALIDATION_ERROR"
	CodeUnauthorized          = "UNAUTHORIZED"
	CodeForbidden             = "FORBIDDEN"
	CodeInsufficientScope     = "INSUFFICIENT_SCOPE"
	CodeNotFound              = "NOT_FOUND"
	CodeUnknownAgent          = "UNKNOWN_AGENT"
	CodeConflict              = "CONFLICT"
	CodeInsufficientCredits   = "INSUFFICIENT_CREDITS"
	CodeRateLimited           = "RATE_LIMITED"
	CodeConcurrentGenLimit    = "CONCURRENT_GENERATION_LIMIT"
	CodeInternalError         = "INTERNAL_ERROR"
	CodeAIProviderError       = "AI_PROVIDER_ERROR"
	CodeGenerationTimeout     = "GENERATION_TIMEOUT"
	CodeServerRestarting      = "SERVER_RESTARTING"
	CodeStreamNotFound        = "STREAM_NOT_FOUND"
	CodeStreamError           = "STREAM_ERROR"
	CodeSessionDeleted        = "SESSION_DELETED"

	// Client-side codes (not from API).
	CodeInvalidAgentType      = "INVALID_AGENT_TYPE"
	CodeMalformedResponse     = "MALFORMED_RESPONSE"
	CodeInteractionHandlerReq = "INTERACTION_HANDLER_REQUIRED"
	CodeStreamIncomplete      = "STREAM_INCOMPLETE"
	CodeAborted               = "ABORTED"
	CodePollTimeout           = "POLL_TIMEOUT"
)

// retryableCodes is the set of error codes that should trigger a retry.
var retryableCodes = map[string]bool{
	CodeRateLimited:        true,
	CodeConcurrentGenLimit: true,
	CodeGenerationTimeout:  true,
	CodeServerRestarting:   true,
	CodeStreamError:        true,
}

// IsRetryableCode returns true if the given error code should trigger a retry.
func IsRetryableCode(code string) bool {
	return retryableCodes[code]
}

// IsRetryableStatus returns true if the HTTP status code should trigger a retry.
func IsRetryableStatus(status int) bool {
	return status == http.StatusBadGateway ||
		status == http.StatusServiceUnavailable ||
		status == http.StatusGatewayTimeout
}

// ParseErrorResponse parses an error response body into an APIError.
func ParseErrorResponse(resp *http.Response) *APIError {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &APIError{
			HTTPStatus: resp.StatusCode,
			Code:       CodeMalformedResponse,
			Message:    fmt.Sprintf("failed to read error response: %v", err),
		}
	}

	var raw errorResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return &APIError{
			HTTPStatus: resp.StatusCode,
			Code:       CodeMalformedResponse,
			Message:    fmt.Sprintf("unparseable error response: %s", string(body)),
		}
	}

	return &APIError{
		HTTPStatus: resp.StatusCode,
		Code:       raw.Error.Code,
		Message:    raw.Error.Message,
		Hint:       raw.Error.Hint,
		Details:    raw.Error.Details,
		RequestID:  raw.RequestID,
		Path:       raw.Path,
	}
}
