package client

import (
	"fmt"
	"net/http"
	"time"
)

// Client is the HTTP client for the LLM Conveyors API.
type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
	MaxRetries int
	Debug      bool
	UserAgent  string
}

// New creates a new API client.
func New(apiKey, baseURL string) (*Client, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required (set LLMC_API_KEY or use --api-key)")
	}
	if len(apiKey) < 6 || apiKey[:5] != "llmc_" {
		return nil, fmt.Errorf("API key must start with 'llmc_' prefix")
	}

	return &Client{
		BaseURL: baseURL,
		APIKey:  apiKey,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		MaxRetries: 3,
		UserAgent:  "llmconveyors-go/dev",
	}, nil
}
