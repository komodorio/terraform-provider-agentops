// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewInjectsBearerAndUserAgent(t *testing.T) {
	var gotAuth, gotUA, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotUA = r.Header.Get("User-Agent")
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("[]"))
	}))
	defer srv.Close()

	c, err := New(srv.URL, "secret-key", "1.2.3")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	resp, err := c.Gen.ApiKeysListApiKeysEndpointWithResponse(context.Background())
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.HTTPResponse.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.HTTPResponse.StatusCode)
	}
	if gotAuth != "Bearer secret-key" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer secret-key")
	}
	if gotUA != "terraform-provider-agentops/1.2.3" {
		t.Errorf("User-Agent = %q, want %q", gotUA, "terraform-provider-agentops/1.2.3")
	}
	if gotPath != "/api/v1/api-keys" {
		t.Errorf("path = %q, want %q", gotPath, "/api/v1/api-keys")
	}
}

func TestCheck(t *testing.T) {
	if err := Check(&http.Response{StatusCode: 204}, nil); err != nil {
		t.Errorf("Check(204) = %v, want nil", err)
	}
	if err := Check(nil, nil); err == nil {
		t.Error("Check(nil) = nil, want error")
	}

	err := Check(&http.Response{StatusCode: 404}, []byte(`{"detail":"nope"}`))
	if err == nil {
		t.Fatal("Check(404) = nil, want error")
	}
	if !IsNotFound(err) {
		t.Errorf("IsNotFound(%v) = false, want true", err)
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("error type = %T, want *APIError", err)
	}
	if apiErr.StatusCode != 404 {
		t.Errorf("StatusCode = %d, want 404", apiErr.StatusCode)
	}

	if IsNotFound(Check(&http.Response{StatusCode: 500}, nil)) {
		t.Error("IsNotFound(500) = true, want false")
	}
}
