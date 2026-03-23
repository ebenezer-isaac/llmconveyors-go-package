package client

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestRetryConfig_CalculateBackoff(t *testing.T) {
	rc := RetryConfig{
		BaseDelay: 1 * time.Second,
		MaxDelay:  30 * time.Second,
		MaxJitter: 0, // No jitter — safe now with guard.
	}

	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{0, 1 * time.Second},
		{1, 2 * time.Second},
		{2, 4 * time.Second},
		{3, 8 * time.Second},
		{4, 16 * time.Second},
		{5, 30 * time.Second}, // Capped at MaxDelay.
		{10, 30 * time.Second},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := rc.CalculateBackoff(tt.attempt)
			if got != tt.want {
				t.Errorf("CalculateBackoff(%d)=%v, want %v", tt.attempt, got, tt.want)
			}
		})
	}
}

func TestRetryConfig_CalculateBackoff_WithJitter(t *testing.T) {
	rc := RetryConfig{
		BaseDelay: 1 * time.Second,
		MaxDelay:  30 * time.Second,
		MaxJitter: 1000 * time.Millisecond,
	}

	for i := 0; i < 100; i++ {
		got := rc.CalculateBackoff(0)
		if got < 1*time.Second || got > 2*time.Second {
			t.Errorf("CalculateBackoff(0)=%v, want between 1s and 2s", got)
		}
	}
}

func TestRetryConfig_CalculateBackoff_ZeroJitter(t *testing.T) {
	// Verify no panic with MaxJitter=0.
	rc := RetryConfig{
		BaseDelay: 1 * time.Second,
		MaxDelay:  30 * time.Second,
		MaxJitter: 0,
	}

	got := rc.CalculateBackoff(0)
	if got != 1*time.Second {
		t.Errorf("CalculateBackoff(0) with zero jitter=%v, want 1s", got)
	}
}

func TestParseRetryAfter_Seconds(t *testing.T) {
	resp := &http.Response{Header: http.Header{}}
	resp.Header.Set("Retry-After", "30")

	got := ParseRetryAfter(resp)
	if got != 30*time.Second {
		t.Errorf("ParseRetryAfter()=%v, want 30s", got)
	}
}

func TestParseRetryAfter_Missing(t *testing.T) {
	resp := &http.Response{Header: http.Header{}}

	got := ParseRetryAfter(resp)
	if got != defaultRetryAfter {
		t.Errorf("ParseRetryAfter()=%v, want %v (default)", got, defaultRetryAfter)
	}
}

func TestParseRetryAfter_InvalidValue(t *testing.T) {
	resp := &http.Response{Header: http.Header{}}
	resp.Header.Set("Retry-After", "not-a-number")

	got := ParseRetryAfter(resp)
	if got != defaultRetryAfter {
		t.Errorf("ParseRetryAfter()=%v, want %v (default for unparseable)", got, defaultRetryAfter)
	}
}

func TestParseRetryAfter_Zero(t *testing.T) {
	resp := &http.Response{Header: http.Header{}}
	resp.Header.Set("Retry-After", "0")

	got := ParseRetryAfter(resp)
	// 0 is not > 0, so falls through to default.
	if got != defaultRetryAfter {
		t.Errorf("ParseRetryAfter()=%v, want %v (default for zero)", got, defaultRetryAfter)
	}
}

func TestParseRateLimitHeaders(t *testing.T) {
	resp := &http.Response{Header: http.Header{}}
	resp.Header.Set("X-RateLimit-Limit", "100")
	resp.Header.Set("X-RateLimit-Remaining", "42")
	resp.Header.Set("X-RateLimit-Reset", "1700000000")

	info := ParseRateLimitHeaders(resp)

	if info.Limit != 100 {
		t.Errorf("Limit=%d, want 100", info.Limit)
	}
	if info.Remaining != 42 {
		t.Errorf("Remaining=%d, want 42", info.Remaining)
	}
	if info.Reset.Unix() != 1700000000 {
		t.Errorf("Reset=%v, want epoch 1700000000", info.Reset)
	}
}

func TestParseRateLimitHeaders_Missing(t *testing.T) {
	resp := &http.Response{Header: http.Header{}}

	info := ParseRateLimitHeaders(resp)
	if info.Limit != 0 || info.Remaining != 0 {
		t.Errorf("expected zero values for missing headers, got Limit=%d Remaining=%d", info.Limit, info.Remaining)
	}
}

func TestShouldRetry_APIError(t *testing.T) {
	retryable := &APIError{Code: CodeRateLimited, HTTPStatus: 429}
	if !ShouldRetry(retryable) {
		t.Error("expected ShouldRetry=true for RATE_LIMITED")
	}

	nonRetryable := &APIError{Code: CodeNotFound, HTTPStatus: 404}
	if ShouldRetry(nonRetryable) {
		t.Error("expected ShouldRetry=false for NOT_FOUND")
	}
}

func TestShouldRetry_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if ShouldRetry(ctx.Err()) {
		t.Error("expected ShouldRetry=false for context.Canceled")
	}
}

func TestShouldRetry_ContextDeadlineExceeded(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()
	<-ctx.Done()
	if ShouldRetry(ctx.Err()) {
		t.Error("expected ShouldRetry=false for context.DeadlineExceeded")
	}
}

func TestShouldRetry_AIProviderErrorNotRetryableByCode(t *testing.T) {
	// Per official docs, AI_PROVIDER_ERROR is NOT in retryable codes.
	// But HTTP 502 IS retryable by status.
	byCode := &APIError{Code: CodeAIProviderError, HTTPStatus: 200}
	if ShouldRetry(byCode) {
		t.Error("expected ShouldRetry=false for AI_PROVIDER_ERROR with non-retryable status")
	}

	byStatus := &APIError{Code: CodeAIProviderError, HTTPStatus: 502}
	if !ShouldRetry(byStatus) {
		t.Error("expected ShouldRetry=true for AI_PROVIDER_ERROR with 502 status")
	}
}

func TestSleep_Cancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := Sleep(ctx, 10*time.Second)
	if err != context.Canceled {
		t.Errorf("Sleep()=%v, want context.Canceled", err)
	}
}

func TestSleep_CompletesNormally(t *testing.T) {
	err := Sleep(context.Background(), 1*time.Millisecond)
	if err != nil {
		t.Errorf("Sleep()=%v, want nil", err)
	}
}
