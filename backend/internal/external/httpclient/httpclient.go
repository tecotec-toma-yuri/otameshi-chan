// Package httpclient provides shared HTTP plumbing (client construction,
// request sending, status/body handling) for external/* API clients.
package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// BaseClient holds the shared HTTP client and base URL for an external API client.
type BaseClient struct {
	BaseURL string
	HTTP    *http.Client
}

func NewBaseClient(baseURL string, timeout time.Duration) BaseClient {
	return BaseClient{
		BaseURL: baseURL,
		HTTP:    &http.Client{Timeout: timeout},
	}
}

// Post sends body to an absolute URL with the given content type and an optional bearer token.
func (b BaseClient) Post(ctx context.Context, url, contentType string, body io.Reader, bearerToken string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+bearerToken)
	}

	return b.HTTP.Do(req)
}

// PostJSON marshals payload and POSTs it to BaseURL+path, optionally with a bearer token.
func (b BaseClient) PostJSON(ctx context.Context, path string, payload any, bearerToken string) (*http.Response, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.BaseURL+path, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+bearerToken)
	}
	return b.HTTP.Do(req)
}

// ReadBody validates the response status is 200 and returns the raw body.
func ReadBody(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(data))
	}
	return data, nil
}

// DecodeJSON validates the response status is 200 and decodes the JSON body into v.
func DecodeJSON(resp *http.Response, v any) error {
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}
	return json.NewDecoder(resp.Body).Decode(v)
}
