// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

// Package client is the thin, hand-written layer over the generated AgentOps
// control-plane client (package gen). It wires up the base URL, Bearer auth, a
// retrying HTTP transport, and a shared non-2xx -> error helper. Everything
// resource-specific lives in the provider package and calls c.Gen.<Op>WithResponse.
package client

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/go-retryablehttp"

	"github.com/komodorio/terraform-provider-agentops/internal/client/gen"
)

// DefaultEndpoint is the AgentOps production control-plane base URL. Override
// with the provider `endpoint` attribute or the AGENTOPS_ENDPOINT env var (e.g.
// https://staging.agentops.komodor.com for staging or a self-hosted URL).
const DefaultEndpoint = "https://agentops.komodor.com"

// Client wraps the generated ClientWithResponses. Resources reach the API via
// the embedded Gen; there is no bespoke per-resource HTTP.
type Client struct {
	Gen *gen.ClientWithResponses
}

// New builds a Client pointed at endpoint, authenticating every request with the
// given API key. userAgent is folded into the User-Agent header for server-side
// attribution (pass the provider version).
func New(endpoint, apiKey, userAgent string) (*Client, error) {
	retry := retryablehttp.NewClient()
	// Terraform emits its own structured logs; keep retryablehttp quiet.
	retry.Logger = nil
	// Retry transient failures: connection errors, 429, and 5xx (the default
	// CheckRetry policy). Retry-After is honored for 429/503.
	retry.RetryMax = 4

	gc, err := gen.NewClientWithResponses(
		endpoint,
		gen.WithHTTPClient(retry.StandardClient()),
		gen.WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
			req.Header.Set("Authorization", "Bearer "+apiKey)
			return nil
		}),
		gen.WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
			req.Header.Set("User-Agent", "terraform-provider-agentops/"+userAgent)
			return nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("building AgentOps client: %w", err)
	}
	return &Client{Gen: gc}, nil
}

// APIError is returned for any non-2xx response. It carries the status code and
// raw response body so callers (and users) get an actionable message.
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("agentops API returned HTTP %d", e.StatusCode)
	}
	return fmt.Sprintf("agentops API returned HTTP %d: %s", e.StatusCode, e.Body)
}

// Check turns a generated response's HTTP result into an error when the status
// is outside the 2xx range. Pass the response's HTTPResponse and Body fields:
//
//	resp, err := c.Gen.TriggersGetTriggerEndpointWithResponse(ctx, id)
//	if err != nil { ... }
//	if err := client.Check(resp.HTTPResponse, resp.Body); err != nil { ... }
func Check(httpResp *http.Response, body []byte) error {
	if httpResp == nil {
		return fmt.Errorf("agentops API returned no response")
	}
	if httpResp.StatusCode >= 200 && httpResp.StatusCode < 300 {
		return nil
	}
	return &APIError{StatusCode: httpResp.StatusCode, Body: string(body)}
}

// IsNotFound reports whether err is an APIError with a 404 status. Resources use
// it in Read to detect out-of-band deletion and drop the resource from state.
func IsNotFound(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.StatusCode == http.StatusNotFound
	}
	return false
}
