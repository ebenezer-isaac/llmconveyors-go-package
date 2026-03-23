package client

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"time"
)

// RetryConfig controls retry behavior.
type RetryConfig struct {
	MaxRetries int
	BaseDelay  time.Duration
	MaxDelay   time.Duration
	MaxJitter  time.Duration
}

// DefaultRetryConfig returns sensible retry defaults per official docs.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries: 3,
		BaseDelay:  1 * time.Second,
		MaxDelay:   30 * time.Second,
		MaxJitter:  1000 * time.Millisecond,
	}
}

// CalculateBackoff returns the delay for the given attempt using exponential backoff with jitter.
// delay = min(baseDelay * 2^attempt, maxDelay) + random(0, maxJitter)
func (rc RetryConfig) CalculateBackoff(attempt int) time.Duration {
	exp := math.Pow(2, float64(attempt))
	delay := time.Duration(float64(rc.BaseDelay) * exp)
	if delay > rc.MaxDelay {
		delay = rc.MaxDelay
	}
	if rc.MaxJitter > 0 {
		jitter := time.Duration(rand.Int63n(int64(rc.MaxJitter)))
		delay += jitter
	}
	return delay
}

// defaultRetryAfter is the default wait time when a 429 has no Retry-After header.
const defaultRetryAfter = 60 * time.Second

// ParseRetryAfter extracts the Retry-After header value as a duration.
// Supports both seconds (integer) and HTTP-date formats.
// Returns defaultRetryAfter (60s) if the header is absent, per official docs.
func ParseRetryAfter(resp *http.Response) time.Duration {
	val := resp.Header.Get("Retry-After")
	if val == "" {
		return defaultRetryAfter
	}

	// Try integer seconds first.
	if seconds, err := strconv.Atoi(val); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}

	// Try HTTP-date format.
	if t, err := http.ParseTime(val); err == nil {
		d := time.Until(t)
		if d > 0 {
			return d
		}
	}

	return defaultRetryAfter
}

// RateLimitInfo contains parsed rate limit headers.
type RateLimitInfo struct {
	Limit     int
	Remaining int
	Reset     time.Time
}

// ParseRateLimitHeaders extracts X-RateLimit-* headers from the response.
func ParseRateLimitHeaders(resp *http.Response) RateLimitInfo {
	info := RateLimitInfo{}

	if v := resp.Header.Get("X-RateLimit-Limit"); v != "" {
		info.Limit, _ = strconv.Atoi(v)
	}
	if v := resp.Header.Get("X-RateLimit-Remaining"); v != "" {
		info.Remaining, _ = strconv.Atoi(v)
	}
	if v := resp.Header.Get("X-RateLimit-Reset"); v != "" {
		if epoch, err := strconv.ParseInt(v, 10, 64); err == nil {
			info.Reset = time.Unix(epoch, 0)
		}
	}

	return info
}

// ShouldRetry determines if a request should be retried based on the error.
// Context errors (Canceled, DeadlineExceeded) are never retried.
func ShouldRetry(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.IsRetryable()
	}
	// Network errors are retryable.
	return true
}

// Sleep waits for the specified duration, respecting context cancellation.
func Sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
