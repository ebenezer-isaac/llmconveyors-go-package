package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is the HTTP client for the LLM Conveyors API.
type Client struct {
	BaseURL     string
	APIKey      string
	HTTPClient  *http.Client
	Retry       RetryConfig
	Debug       bool
	UserAgent   string
	DebugWriter io.Writer
}

// New creates a new API client.
func New(apiKey, baseURL string, opts ...Option) (*Client, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required (set LLMC_API_KEY or use --api-key)")
	}
	if len(apiKey) < 6 || apiKey[:5] != "llmc_" {
		return nil, fmt.Errorf("API key must start with 'llmc_' prefix")
	}

	c := &Client{
		BaseURL: baseURL,
		APIKey:  apiKey,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		Retry:     DefaultRetryConfig(),
		UserAgent: "llmconveyors-go/dev",
	}

	for _, opt := range opts {
		opt(c)
	}

	return c, nil
}

// Option configures the client.
type Option func(*Client)

// WithTimeout sets the HTTP client timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		c.HTTPClient.Timeout = d
	}
}

// WithMaxRetries sets the maximum number of retries.
func WithMaxRetries(n int) Option {
	return func(c *Client) {
		c.Retry.MaxRetries = n
	}
}

// WithDebug enables debug logging.
func WithDebug(w io.Writer) Option {
	return func(c *Client) {
		c.Debug = true
		c.DebugWriter = w
	}
}

// WithUserAgent sets the User-Agent header.
func WithUserAgent(ua string) Option {
	return func(c *Client) {
		c.UserAgent = ua
	}
}

// Get performs a GET request and decodes the response into result.
func (c *Client) Get(ctx context.Context, path string, result interface{}) error {
	return c.do(ctx, http.MethodGet, path, nil, result)
}

// Post performs a POST request with a JSON body and decodes the response.
func (c *Client) Post(ctx context.Context, path string, body interface{}, result interface{}) error {
	return c.do(ctx, http.MethodPost, path, body, result)
}

// Put performs a PUT request with a JSON body and decodes the response.
func (c *Client) Put(ctx context.Context, path string, body interface{}, result interface{}) error {
	return c.do(ctx, http.MethodPut, path, body, result)
}

// Delete performs a DELETE request and decodes the response.
func (c *Client) Delete(ctx context.Context, path string, result interface{}) error {
	return c.do(ctx, http.MethodDelete, path, nil, result)
}

// GetRaw performs a GET and returns the raw *http.Response (caller must close body).
// Accepts optional headers (e.g., Last-Event-ID for SSE reconnection).
func (c *Client) GetRaw(ctx context.Context, path string, headers ...http.Header) (*http.Response, error) {
	return c.doRaw(ctx, http.MethodGet, path, nil, mergeHeaders(headers...))
}

// PostMultipart performs a POST with a pre-built body (e.g., multipart) and returns decoded result.
func (c *Client) PostMultipart(ctx context.Context, path string, contentType string, body io.Reader, result interface{}) error {
	return c.doMultipart(ctx, path, contentType, body, result)
}

func mergeHeaders(headers ...http.Header) http.Header {
	if len(headers) == 0 {
		return nil
	}
	merged := http.Header{}
	for _, h := range headers {
		for k, vals := range h {
			for _, v := range vals {
				merged.Add(k, v)
			}
		}
	}
	return merged
}

// attemptResult captures the outcome of a single HTTP request attempt.
type attemptResult struct {
	shouldRetry bool
	retryAfter  time.Duration // Non-zero only for 429 with Retry-After header.
	err         error
}

// doAttempt executes a single HTTP request attempt and handles the response.
func (c *Client) doAttempt(ctx context.Context, method, path string, body interface{}, result interface{}) attemptResult {
	resp, err := c.doRaw(ctx, method, path, body, nil)
	if err != nil {
		return attemptResult{shouldRetry: ShouldRetry(err), err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		apiErr := ParseErrorResponse(resp)

		// Capture Retry-After for 429 — let caller decide whether to sleep.
		if resp.StatusCode == http.StatusTooManyRequests {
			ra := ParseRetryAfter(resp)
			return attemptResult{shouldRetry: true, retryAfter: ra, err: apiErr}
		}

		return attemptResult{shouldRetry: apiErr.IsRetryable(), err: apiErr}
	}

	if result == nil {
		return attemptResult{}
	}

	return attemptResult{err: c.decodeResponse(resp.Body, result)}
}

func (c *Client) do(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	var lastErr error

	for attempt := 0; attempt <= c.Retry.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := c.Retry.CalculateBackoff(attempt - 1)
			if err := Sleep(ctx, delay); err != nil {
				return err
			}
		}

		ar := c.doAttempt(ctx, method, path, body, result)
		if ar.err != nil {
			lastErr = ar.err
			if ar.shouldRetry && attempt < c.Retry.MaxRetries {
				// Sleep for Retry-After if present (429).
				if ar.retryAfter > 0 {
					if err := Sleep(ctx, ar.retryAfter); err != nil {
						return err
					}
				}
				continue
			}
			return ar.err
		}

		return nil
	}

	return lastErr
}

func (c *Client) doRaw(ctx context.Context, method, path string, body interface{}, extraHeaders http.Header) (*http.Response, error) {
	url := c.BaseURL + path

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshaling request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("X-API-Key", c.APIKey)
	req.Header.Set("User-Agent", c.UserAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	// Apply extra headers (e.g., Last-Event-ID).
	for k, vals := range extraHeaders {
		for _, v := range vals {
			req.Header.Set(k, v)
		}
	}

	if c.Debug && c.DebugWriter != nil {
		fmt.Fprintf(c.DebugWriter, ">> %s %s\n", method, url)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if c.Debug && c.DebugWriter != nil {
		fmt.Fprintf(c.DebugWriter, "<< %d %s\n", resp.StatusCode, resp.Status)
	}

	return resp, nil
}

func (c *Client) doMultipart(ctx context.Context, path, contentType string, body io.Reader, result interface{}) error {
	url := c.BaseURL + path

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("X-API-Key", c.APIKey)
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Content-Type", contentType)

	if c.Debug && c.DebugWriter != nil {
		fmt.Fprintf(c.DebugWriter, ">> POST %s (multipart)\n", url)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if c.Debug && c.DebugWriter != nil {
		fmt.Fprintf(c.DebugWriter, "<< %d %s\n", resp.StatusCode, resp.Status)
	}

	if resp.StatusCode >= 400 {
		return ParseErrorResponse(resp)
	}

	if result == nil {
		return nil
	}

	return c.decodeResponse(resp.Body, result)
}

// decodeResponse unwraps the API envelope and decodes Data into result.
func (c *Client) decodeResponse(body io.Reader, result interface{}) error {
	raw, err := io.ReadAll(body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	// First try to unmarshal the full envelope.
	var envelope struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return &APIError{
			Code:    CodeMalformedResponse,
			Message: fmt.Sprintf("invalid JSON response: %v", err),
		}
	}

	if !envelope.Success {
		return &APIError{
			Code:    CodeMalformedResponse,
			Message: "API returned success=false without error details",
		}
	}

	// Decode the data payload into the caller's struct.
	if err := json.Unmarshal(envelope.Data, result); err != nil {
		return &APIError{
			Code:    CodeMalformedResponse,
			Message: fmt.Sprintf("failed to decode response data: %v", err),
		}
	}

	return nil
}
