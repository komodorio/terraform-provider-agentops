// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

// Package client is the thin, hand-written layer over the generated AgentOps
// control-plane client (package gen). It wires up the base URL, Bearer auth, a
// retrying HTTP transport, and a shared non-2xx -> error helper. Everything
// resource-specific lives in the provider package and calls c.Gen.<Op>WithResponse.
package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/hashicorp/go-retryablehttp"

	"github.com/komodorio/terraform-provider-agentops/internal/client/gen"
)

// DefaultEndpoint is the AgentOps production control-plane base URL. Override
// with the provider `endpoint` attribute or the AGENTOPS_ENDPOINT env var (e.g.
// https://staging.agentops.komodor.com for staging or a self-hosted URL).
const DefaultEndpoint = "https://agentops.komodor.com"

// Client wraps the generated ClientWithResponses. Resources reach the API via the
// embedded Gen. Post exists for the few routes the vendored spec does not carry
// yet, so a resource still never builds its own HTTP client.
type Client struct {
	Gen *gen.ClientWithResponses

	endpoint   string
	apiKey     string
	userAgent  string
	httpClient *http.Client
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

	c := &Client{
		endpoint:   strings.TrimSuffix(endpoint, "/"),
		apiKey:     apiKey,
		userAgent:  userAgent,
		httpClient: retry.StandardClient(),
	}
	gc, err := gen.NewClientWithResponses(
		endpoint,
		gen.WithHTTPClient(c.httpClient),
		gen.WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
			c.setHeaders(req)
			return nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("building AgentOps client: %w", err)
	}
	c.Gen = gc
	return c, nil
}

// setHeaders applies the auth and attribution every request carries, so the
// generated calls and Post cannot drift apart.
func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("User-Agent", "terraform-provider-agentops/"+c.userAgent)
}

// Post issues a bodyless POST to path (rooted, e.g. "/api/v1/…"), with the same
// auth, User-Agent and retry behaviour as the generated calls. It is for routes
// the vendored OpenAPI spec has not caught up with; anything the spec covers must
// go through Gen instead.
func (c *Client) Post(ctx context.Context, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+path, nil)
	if err != nil {
		return err
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return Check(resp, body)
}

// Do turns one raw generated call — c.Gen.<Op>(...), not <Op>WithResponse — into
// its body plus the same non-2xx error Check produces. Reach for it when the
// endpoint declares an error status whose real body does not match the schema:
// oapi-codegen's typed wrapper discards the whole response, status and body
// included, the moment such a body fails to unmarshal, and FastAPI answers
// several preconditions with a 422 carrying a plain-string `detail` rather than
// the validation-error list the spec declares. Through the wrapper those surface
// as an opaque json error with no status attached; through Do they surface as the
// APIError they are.
func Do(resp *http.Response, err error) ([]byte, error) {
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, readErr
	}
	return body, Check(resp, body)
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
	return isStatus(err, http.StatusNotFound)
}

// IsConflict reports whether err is an APIError with a 409 status. What a 409
// means is per-endpoint, so callers must match on the response body before acting
// on one.
func IsConflict(err error) bool {
	return isStatus(err, http.StatusConflict)
}

// IsUnprocessable reports whether err is an APIError with a 422 status. FastAPI
// reserves 422 for request-validation errors, so an endpoint that answers one for
// an unmet precondition is doing it in passing; callers must match on the body
// before acting on one.
func IsUnprocessable(err error) bool {
	return isStatus(err, http.StatusUnprocessableEntity)
}

func isStatus(err error, code int) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == code
}
