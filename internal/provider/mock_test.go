// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sync"
	"testing"
)

// mockServer is an in-memory AgentOps control-plane used by acceptance tests so
// they exercise the full Terraform lifecycle (plan/apply/import/destroy) without
// a live backend or secrets. It faithfully echoes create/update request fields
// the way the real API does, which is what makes drift-free plans meaningful.
type mockServer struct {
	*httptest.Server
	mu       sync.Mutex
	triggers map[string]map[string]any
	apiKeys  map[string]map[string]any
	seq      int
}

var (
	triggerIDRe = regexp.MustCompile(`^/api/v1/triggers/([^/]+)$`)
	apiKeyIDRe  = regexp.MustCompile(`^/api/v1/api-keys/([^/]+)$`)
)

func newMockServer(t *testing.T) *mockServer {
	t.Helper()
	m := &mockServer{
		triggers: map[string]map[string]any{},
		apiKeys:  map[string]map[string]any{},
	}
	m.Server = httptest.NewServer(http.HandlerFunc(m.handle))
	t.Cleanup(m.Close)
	return m
}

func (m *mockServer) nextID(prefix string) string {
	m.seq++
	return fmt.Sprintf("%s_%d", prefix, m.seq)
}

func (m *mockServer) handle(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"detail": "missing bearer token"})
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	switch {
	case r.URL.Path == "/api/v1/triggers" && r.Method == http.MethodPost:
		m.createTrigger(w, r)
	case triggerIDRe.MatchString(r.URL.Path):
		m.triggerByID(w, r, triggerIDRe.FindStringSubmatch(r.URL.Path)[1])
	case r.URL.Path == "/api/v1/api-keys" && r.Method == http.MethodPost:
		m.createAPIKey(w, r)
	case r.URL.Path == "/api/v1/api-keys" && r.Method == http.MethodGet:
		m.listAPIKeys(w)
	case apiKeyIDRe.MatchString(r.URL.Path) && r.Method == http.MethodDelete:
		delete(m.apiKeys, apiKeyIDRe.FindStringSubmatch(r.URL.Path)[1])
		writeJSON(w, http.StatusOK, map[string]any{})
	default:
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": "not found: " + r.Method + " " + r.URL.Path})
	}
}

func (m *mockServer) createTrigger(w http.ResponseWriter, r *http.Request) {
	body := decode(r)
	id := m.nextID("trg")
	t := map[string]any{
		"trigger_id": id,
		"header":     "X-Signature",
		"is_enabled": true,
		"created_at": "2026-07-13T00:00:00Z",
		"updated_at": "2026-07-13T00:00:00Z",
	}
	for _, k := range []string{"name", "description", "target_id", "target_type", "webhook_type", "header", "is_enabled", "signing_credential_id"} {
		if v, ok := body[k]; ok && v != nil {
			t[k] = v
		}
	}
	m.triggers[id] = t

	// Create returns the token; reads never do.
	resp := cloneMap(t)
	resp["token"] = "tok_" + id
	writeJSON(w, http.StatusCreated, resp)
}

func (m *mockServer) triggerByID(w http.ResponseWriter, r *http.Request, id string) {
	t, ok := m.triggers[id]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": "trigger not found"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, t)
	case http.MethodPatch:
		for k, v := range decode(r) {
			if v != nil {
				t[k] = v
			}
		}
		m.triggers[id] = t
		writeJSON(w, http.StatusOK, t)
	case http.MethodDelete:
		delete(m.triggers, id)
		w.WriteHeader(http.StatusNoContent)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{})
	}
}

func (m *mockServer) createAPIKey(w http.ResponseWriter, r *http.Request) {
	body := decode(r)
	id := m.nextID("key")
	scopes := body["scopes"]
	if scopes == nil {
		scopes = []any{}
	}
	boundTo := body["bound_to"]
	if boundTo == nil {
		boundTo = "service_account"
	}
	k := map[string]any{
		"id":           id,
		"name":         body["name"],
		"bound_to":     boundTo,
		"scopes":       scopes,
		"status":       "active",
		"principal_id": "prn_" + id,
		"created_at":   "2026-07-13T00:00:00Z",
	}
	if v, ok := body["expires_at"]; ok && v != nil {
		k["expires_at"] = v
	}
	m.apiKeys[id] = k

	resp := cloneMap(k)
	resp["key"] = "sk_" + id
	writeJSON(w, http.StatusCreated, resp)
}

func (m *mockServer) listAPIKeys(w http.ResponseWriter) {
	out := make([]map[string]any, 0, len(m.apiKeys))
	for _, k := range m.apiKeys {
		out = append(out, k)
	}
	writeJSON(w, http.StatusOK, out)
}

func decode(r *http.Request) map[string]any {
	var body map[string]any
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}
	if body == nil {
		body = map[string]any{}
	}
	return body
}

func cloneMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in)+1)
	for k, v := range in {
		out[k] = v
	}
	return out
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// mockProviderConfig renders a provider block pointed at the mock server.
func mockProviderConfig(endpoint string) string {
	return fmt.Sprintf(`
provider "agentops" {
  endpoint = %q
  api_key  = "test-key"
}
`, endpoint)
}
