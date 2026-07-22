// Package api provides shared HTTP plumbing for the searcher providers.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/labi-le/chiasma/pkg/api/searcher"
)

// DefaultTimeout is the request timeout applied to provider HTTP clients when
// the caller does not inject its own *http.Client.
const DefaultTimeout = 30 * time.Second

// Client returns c when non-nil, otherwise a client with a sane default timeout.
func Client(c *http.Client) *http.Client {
	if c != nil {
		return c
	}
	return &http.Client{Timeout: DefaultTimeout}
}

// FetchJSON executes req, requires a 2xx status, and decodes the body into v.
func FetchJSON(client *http.Client, req *http.Request, v any) error {
	//nolint:gosec // G704: intentional outbound HTTP to provider search APIs; the
	// URL is built from provider constants with an escaped user query, not arbitrary taint.
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	if decErr := json.NewDecoder(resp.Body).Decode(v); decErr != nil {
		return fmt.Errorf("decode json: %w", decErr)
	}
	return nil
}

// Download fetches url and returns an Image whose Size reports the ACTUAL
// decoded dimensions of the downloaded bytes (not any nominal source size).
//
//nolint:ireturn // seam: providers depend on the searcher.Image abstraction, not a concrete type.
func Download(ctx context.Context, client *http.Client, url string) (searcher.Image, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	img, err := searcher.DetectSize(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("detect size: %w", err)
	}
	return img, nil
}
