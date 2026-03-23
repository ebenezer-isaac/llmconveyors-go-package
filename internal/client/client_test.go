package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNew_ValidKey(t *testing.T) {
	c, err := New("llmc_test123", "https://api.example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.APIKey != "llmc_test123" {
		t.Errorf("got APIKey=%q, want %q", c.APIKey, "llmc_test123")
	}
	if c.BaseURL != "https://api.example.com" {
		t.Errorf("got BaseURL=%q, want %q", c.BaseURL, "https://api.example.com")
	}
}

func TestNew_EmptyKey(t *testing.T) {
	_, err := New("", "https://api.example.com")
	if err == nil {
		t.Fatal("expected error for empty key")
	}
	if !strings.Contains(err.Error(), "API key is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestNew_InvalidPrefix(t *testing.T) {
	_, err := New("sk_badprefix", "https://api.example.com")
	if err == nil {
		t.Fatal("expected error for invalid prefix")
	}
	if !strings.Contains(err.Error(), "llmc_") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestNew_ShortKey(t *testing.T) {
	_, err := New("llmc", "https://api.example.com")
	if err == nil {
		t.Fatal("expected error for short key")
	}
}

func TestNew_WithOptions(t *testing.T) {
	c, err := New("llmc_testkey", "https://api.example.com",
		WithTimeout(10*time.Second),
		WithMaxRetries(5),
		WithUserAgent("test-agent/1.0"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.HTTPClient.Timeout != 10*time.Second {
		t.Errorf("Timeout=%v, want 10s", c.HTTPClient.Timeout)
	}
	if c.Retry.MaxRetries != 5 {
		t.Errorf("MaxRetries=%d, want 5", c.Retry.MaxRetries)
	}
	if c.UserAgent != "test-agent/1.0" {
		t.Errorf("UserAgent=%q, want test-agent/1.0", c.UserAgent)
	}
}

func TestClientGet_Success(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantErr    bool
		wantCode   string
	}{
		{
			name:       "success",
			statusCode: 200,
			body:       `{"success":true,"data":{"status":"ok"}}`,
			wantErr:    false,
		},
		{
			name:       "not found",
			statusCode: 404,
			body:       `{"success":false,"error":{"code":"NOT_FOUND","message":"Job not found"}}`,
			wantErr:    true,
			wantCode:   "NOT_FOUND",
		},
		{
			name:       "unauthorized",
			statusCode: 401,
			body:       `{"success":false,"error":{"code":"UNAUTHORIZED","message":"Invalid API key"}}`,
			wantErr:    true,
			wantCode:   "UNAUTHORIZED",
		},
		{
			name:       "rate limited",
			statusCode: 429,
			body:       `{"success":false,"error":{"code":"RATE_LIMITED","message":"Slow down"}}`,
			wantErr:    true,
			wantCode:   "RATE_LIMITED",
		},
		{
			name:       "server error",
			statusCode: 500,
			body:       `{"success":false,"error":{"code":"INTERNAL_ERROR","message":"Something broke"}}`,
			wantErr:    true,
			wantCode:   "INTERNAL_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if got := r.Header.Get("X-API-Key"); got != "llmc_testkey" {
					t.Errorf("X-API-Key=%q, want %q", got, "llmc_testkey")
				}
				if got := r.Header.Get("User-Agent"); !strings.HasPrefix(got, "llmconveyors-go/") {
					t.Errorf("User-Agent=%q, want prefix %q", got, "llmconveyors-go/")
				}
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			c, _ := New("llmc_testkey", srv.URL, WithMaxRetries(0))

			var result map[string]interface{}
			err := c.Get(context.Background(), "/test", &result)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if apiErr, ok := err.(*APIError); ok && tt.wantCode != "" {
					if apiErr.Code != tt.wantCode {
						t.Errorf("error code=%q, want %q", apiErr.Code, tt.wantCode)
					}
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if result["status"] != "ok" {
					t.Errorf("result status=%v, want %q", result["status"], "ok")
				}
			}
		})
	}
}

func TestClientPost_SendsJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method=%q, want POST", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type=%q, want application/json", ct)
		}

		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["name"] != "test" {
			t.Errorf("body name=%q, want %q", body["name"], "test")
		}

		w.WriteHeader(200)
		w.Write([]byte(`{"success":true,"data":{"id":"123"}}`))
	}))
	defer srv.Close()

	c, _ := New("llmc_testkey", srv.URL, WithMaxRetries(0))

	var result map[string]interface{}
	err := c.Post(context.Background(), "/create", map[string]string{"name": "test"}, &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["id"] != "123" {
		t.Errorf("result id=%v, want %q", result["id"], "123")
	}
}

func TestClientGet_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`not json`))
	}))
	defer srv.Close()

	c, _ := New("llmc_testkey", srv.URL, WithMaxRetries(0))

	var result map[string]interface{}
	err := c.Get(context.Background(), "/test", &result)
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.Code != CodeMalformedResponse {
		t.Errorf("code=%q, want %q", apiErr.Code, CodeMalformedResponse)
	}
}

func TestClientGet_RetryOnServerError(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(502)
			w.Write([]byte(`{"success":false,"error":{"code":"INTERNAL_ERROR","message":"upstream failed"}}`))
			return
		}
		w.WriteHeader(200)
		w.Write([]byte(`{"success":true,"data":{"status":"recovered"}}`))
	}))
	defer srv.Close()

	c, _ := New("llmc_testkey", srv.URL, WithMaxRetries(3))
	c.Retry.BaseDelay = 1 * time.Millisecond
	c.Retry.MaxJitter = 1 * time.Millisecond

	var result map[string]interface{}
	err := c.Get(context.Background(), "/test", &result)
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if attempts != 3 {
		t.Errorf("attempts=%d, want 3", attempts)
	}
	if result["status"] != "recovered" {
		t.Errorf("result status=%v, want %q", result["status"], "recovered")
	}
}

func TestClientGet_RetryWithRetryAfterHeader(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(429)
			w.Write([]byte(`{"success":false,"error":{"code":"RATE_LIMITED","message":"Slow down"}}`))
			return
		}
		w.WriteHeader(200)
		w.Write([]byte(`{"success":true,"data":{"ok":true}}`))
	}))
	defer srv.Close()

	c, _ := New("llmc_testkey", srv.URL, WithMaxRetries(2))
	c.Retry.BaseDelay = 1 * time.Millisecond
	c.Retry.MaxJitter = 1 * time.Millisecond

	var result map[string]interface{}
	err := c.Get(context.Background(), "/test", &result)
	if err != nil {
		t.Fatalf("expected success after retry, got: %v", err)
	}
	if attempts != 2 {
		t.Errorf("attempts=%d, want 2", attempts)
	}
}

func TestClientGet_NoRetryOnNonRetryable(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(404)
		w.Write([]byte(`{"success":false,"error":{"code":"NOT_FOUND","message":"gone"}}`))
	}))
	defer srv.Close()

	c, _ := New("llmc_testkey", srv.URL, WithMaxRetries(3))

	var result map[string]interface{}
	err := c.Get(context.Background(), "/test", &result)
	if err == nil {
		t.Fatal("expected error")
	}
	if attempts != 1 {
		t.Errorf("attempts=%d, want 1 (no retry for 404)", attempts)
	}
}

func TestClientGet_ContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"success":true,"data":{}}`))
	}))
	defer srv.Close()

	c, _ := New("llmc_testkey", srv.URL, WithMaxRetries(0))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var result map[string]interface{}
	err := c.Get(ctx, "/test", &result)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestClientDelete_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method=%q, want DELETE", r.Method)
		}
		w.WriteHeader(200)
		w.Write([]byte(`{"success":true,"data":null}`))
	}))
	defer srv.Close()

	c, _ := New("llmc_testkey", srv.URL, WithMaxRetries(0))
	err := c.Delete(context.Background(), "/resource/123", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientGetRaw_WithHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Last-Event-ID"); got != "42" {
			t.Errorf("Last-Event-ID=%q, want 42", got)
		}
		w.WriteHeader(200)
		w.Write([]byte("event: heartbeat\ndata: {}\n\n"))
	}))
	defer srv.Close()

	c, _ := New("llmc_testkey", srv.URL, WithMaxRetries(0))

	headers := http.Header{}
	headers.Set("Last-Event-ID", "42")
	resp, err := c.GetRaw(context.Background(), "/stream", headers)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()
}

func TestDecodeResponse_SuccessFalse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"success":false,"data":null}`))
	}))
	defer srv.Close()

	c, _ := New("llmc_testkey", srv.URL, WithMaxRetries(0))

	var result map[string]interface{}
	err := c.Get(context.Background(), "/test", &result)
	if err == nil {
		t.Fatal("expected error for success=false")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.Code != CodeMalformedResponse {
		t.Errorf("code=%q, want %q", apiErr.Code, CodeMalformedResponse)
	}
}

func TestDecodeResponse_DataTypeMismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"success":true,"data":"not-an-object"}`))
	}))
	defer srv.Close()

	c, _ := New("llmc_testkey", srv.URL, WithMaxRetries(0))

	var result struct {
		ID int `json:"id"`
	}
	err := c.Get(context.Background(), "/test", &result)
	if err == nil {
		t.Fatal("expected error for data type mismatch")
	}
}
