// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
)

const mockTS = "2026-07-13T00:00:00Z"

// crudResource describes a standard collection the generic mock engine serves:
// POST <collection> (create), GET <collection> (list), and GET/PATCH/DELETE
// <collection>/{id}. Create merges `seed` (computed defaults) with the echoed
// request body, which is what keeps drift-free plans meaningful.
type crudResource struct {
	collection string
	idField    string
	idPrefix   string
	seed       map[string]any
}

var crudRegistry = []crudResource{
	{"/api/v1/credentials", "id", "cred", map[string]any{"status": "active", "created_at": mockTS, "updated_at": mockTS}},
	{"/api/v1/workflows", "workflow_id", "wf", map[string]any{"is_enabled": true, "steps": []any{}, "trigger": map[string]any{}, "created_at": mockTS, "updated_at": mockTS}},
	{"/api/v1/knowledge-bases", "kb_id", "kb", map[string]any{"doc_count": 0, "indexed_count": 0, "created_at": mockTS, "updated_at": mockTS}},
	{"/api/v1/gateway/admin/servers", "id", "srv", map[string]any{"enabled": true}},
	{"/api/v1/gateway/admin/groups", "id", "grp", map[string]any{}},
	{"/api/v1/authz/roles", "id", "role", map[string]any{"builtin": false, "holders": 0}},
	{"/api/v1/authz/policies", "id", "pol", map[string]any{"builtin": false, "created_at": mockTS}},
	{"/api/v1/authz/grants", "id", "grant", map[string]any{"created_at": mockTS}},
}

// mockServer is an in-memory AgentOps control-plane used by acceptance tests so
// they exercise the full Terraform lifecycle (plan/apply/import/destroy) without
// a live backend or secrets. It faithfully echoes create/update request fields
// the way the real API does, which is what makes drift-free plans meaningful.
type mockServer struct {
	*httptest.Server
	mu          sync.Mutex
	triggers    map[string]map[string]any
	apiKeys     map[string]map[string]any
	schedules   map[string]map[string]any
	serviceAccs map[string]map[string]any
	policies    map[string]map[string]any
	conns       map[string]map[string]any
	stores      map[string]map[string]map[string]any // generic CRUD: collection -> id -> record
	seq         int
}

var (
	triggerIDRe  = regexp.MustCompile(`^/api/v1/triggers/([^/]+)$`)
	apiKeyIDRe   = regexp.MustCompile(`^/api/v1/api-keys/([^/]+)$`)
	scheduleRe   = regexp.MustCompile(`^/api/v1/schedules/([^/]+)$`)
	serviceAccRe = regexp.MustCompile(`^/api/v1/accounts/service-accounts/([^/]+)$`)
	policyRe     = regexp.MustCompile(`^/api/v1/gateway/admin/policies/([^/]+)$`)
)

func newMockServer(t *testing.T) *mockServer {
	t.Helper()
	m := &mockServer{
		triggers:    map[string]map[string]any{},
		apiKeys:     map[string]map[string]any{},
		schedules:   map[string]map[string]any{},
		serviceAccs: map[string]map[string]any{},
		policies:    map[string]map[string]any{},
		conns:       map[string]map[string]any{},
		stores:      map[string]map[string]map[string]any{},
	}
	for _, res := range crudRegistry {
		m.stores[res.collection] = map[string]map[string]any{}
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

	case r.URL.Path == "/api/v1/schedules" && r.Method == http.MethodPost:
		m.createSchedule(w, r)
	case scheduleRe.MatchString(r.URL.Path):
		m.scheduleByID(w, r, scheduleRe.FindStringSubmatch(r.URL.Path)[1])

	case r.URL.Path == "/api/v1/accounts/service-accounts" && r.Method == http.MethodPost:
		m.createServiceAccount(w, r)
	case r.URL.Path == "/api/v1/accounts/service-accounts" && r.Method == http.MethodGet:
		m.listMaps(w, m.serviceAccs)
	case serviceAccRe.MatchString(r.URL.Path) && r.Method == http.MethodDelete:
		delete(m.serviceAccs, serviceAccRe.FindStringSubmatch(r.URL.Path)[1])
		writeJSON(w, http.StatusOK, map[string]any{})

	case r.URL.Path == "/api/v1/gateway/admin/policies" && r.Method == http.MethodPost:
		m.createPolicy(w, r)
	case r.URL.Path == "/api/v1/gateway/admin/policies" && r.Method == http.MethodGet:
		m.listMaps(w, m.policies)
	case policyRe.MatchString(r.URL.Path):
		m.policyByID(w, r, policyRe.FindStringSubmatch(r.URL.Path)[1])

	case r.URL.Path == "/api/v1/integrations/connections" && r.Method == http.MethodPost:
		m.createConnection(w, r)
	case strings.HasPrefix(r.URL.Path, "/api/v1/integrations/connections/"):
		m.connectionByID(w, r, strings.TrimPrefix(r.URL.Path, "/api/v1/integrations/connections/"))

	default:
		if m.dispatchCRUD(w, r) {
			return
		}
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": "not found: " + r.Method + " " + r.URL.Path})
	}
}

// dispatchCRUD routes a request to the generic CRUD engine. Returns false if no
// registered collection matches.
func (m *mockServer) dispatchCRUD(w http.ResponseWriter, r *http.Request) bool {
	for i := range crudRegistry {
		res := &crudRegistry[i]
		switch {
		case r.URL.Path == res.collection && r.Method == http.MethodPost:
			m.crudCreate(res, w, r)
			return true
		case r.URL.Path == res.collection && r.Method == http.MethodGet:
			m.listMaps(w, m.stores[res.collection])
			return true
		case strings.HasPrefix(r.URL.Path, res.collection+"/"):
			id := strings.TrimPrefix(r.URL.Path, res.collection+"/")
			// Credential value replacement: PUT /credentials/{id}/value.
			if strings.HasSuffix(id, "/value") {
				m.crudReplaceValue(res, w, strings.TrimSuffix(id, "/value"))
				return true
			}
			m.crudByID(res, w, r, id)
			return true
		}
	}
	return false
}

func (m *mockServer) crudCreate(res *crudResource, w http.ResponseWriter, r *http.Request) {
	body := decode(r)
	id := m.nextID(res.idPrefix)
	rec := cloneMap(res.seed)
	for k, v := range body {
		if v != nil {
			rec[k] = v
		}
	}
	rec[res.idField] = id
	m.stores[res.collection][id] = rec
	writeJSON(w, http.StatusCreated, rec)
}

func (m *mockServer) crudByID(res *crudResource, w http.ResponseWriter, r *http.Request, id string) {
	rec, ok := m.stores[res.collection][id]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": "not found"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, rec)
	case http.MethodPatch, http.MethodPut:
		for k, v := range decode(r) {
			if v != nil {
				rec[k] = v
			}
		}
		m.stores[res.collection][id] = rec
		writeJSON(w, http.StatusOK, rec)
	case http.MethodDelete:
		delete(m.stores[res.collection], id)
		w.WriteHeader(http.StatusNoContent)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{})
	}
}

func (m *mockServer) crudReplaceValue(res *crudResource, w http.ResponseWriter, id string) {
	rec, ok := m.stores[res.collection][id]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": "not found"})
		return
	}
	rec["last_replaced_at"] = mockTS
	writeJSON(w, http.StatusOK, rec)
}

func (m *mockServer) createConnection(w http.ResponseWriter, r *http.Request) {
	body := decode(r)
	id := m.nextID("conn")
	detail := map[string]any{
		"id":                     id,
		"auth_config_key":        "acfg_" + id,
		"external_connection_id": "ext_" + id,
		"display_name":           body["display_name"],
		"provider":               body["provider"],
		"status":                 "connected",
		"created_at":             mockTS,
		"updated_at":             mockTS,
	}
	if v, ok := body["metadata"]; ok && v != nil {
		detail["metadata"] = v
	}
	m.conns[id] = detail
	writeJSON(w, http.StatusCreated, map[string]any{
		"connection_id":   id,
		"auth_config_key": "acfg_" + id,
		"status":          "connected",
	})
}

func (m *mockServer) connectionByID(w http.ResponseWriter, r *http.Request, id string) {
	detail, ok := m.conns[id]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": "connection not found"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, detail)
	case http.MethodDelete:
		delete(m.conns, id)
		w.WriteHeader(http.StatusNoContent)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{})
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
	m.listMaps(w, m.apiKeys)
}

func (m *mockServer) listMaps(w http.ResponseWriter, store map[string]map[string]any) {
	out := make([]map[string]any, 0, len(store))
	for _, v := range store {
		out = append(out, v)
	}
	writeJSON(w, http.StatusOK, out)
}

func (m *mockServer) createSchedule(w http.ResponseWriter, r *http.Request) {
	body := decode(r)
	id := m.nextID("sch")
	s := map[string]any{
		"schedule_id": id,
		"is_enabled":  true,
		"timezone":    "UTC",
		"input":       map[string]any{},
		"created_at":  "2026-07-13T00:00:00Z",
		"updated_at":  "2026-07-13T00:00:00Z",
	}
	for _, k := range []string{"agent_id", "cron_expr", "input", "is_enabled", "timezone", "name", "description"} {
		if v, ok := body[k]; ok && v != nil {
			s[k] = v
		}
	}
	m.schedules[id] = s
	writeJSON(w, http.StatusCreated, s)
}

func (m *mockServer) scheduleByID(w http.ResponseWriter, r *http.Request, id string) {
	s, ok := m.schedules[id]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": "schedule not found"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s)
	case http.MethodPatch:
		for k, v := range decode(r) {
			if v != nil {
				s[k] = v
			}
		}
		m.schedules[id] = s
		writeJSON(w, http.StatusOK, s)
	case http.MethodDelete:
		delete(m.schedules, id)
		w.WriteHeader(http.StatusNoContent)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{})
	}
}

func (m *mockServer) createServiceAccount(w http.ResponseWriter, r *http.Request) {
	body := decode(r)
	id := m.nextID("sa")
	sa := map[string]any{
		"principal_id": id,
		"display_name": body["display_name"],
		"status":       "active",
		"source":       "managed",
		"created_at":   "2026-07-13T00:00:00Z",
	}
	m.serviceAccs[id] = sa
	writeJSON(w, http.StatusCreated, sa)
}

func (m *mockServer) createPolicy(w http.ResponseWriter, r *http.Request) {
	body := decode(r)
	id := m.nextID("pol")
	p := map[string]any{
		"id":          id,
		"name":        "policy-" + id,
		"description": "mock policy",
		"document":    body["document"],
		"enabled":     true,
	}
	for _, k := range []string{"enabled", "target_names", "target_type"} {
		if v, ok := body[k]; ok && v != nil {
			p[k] = v
		}
	}
	m.policies[id] = p
	writeJSON(w, http.StatusCreated, p)
}

func (m *mockServer) policyByID(w http.ResponseWriter, r *http.Request, id string) {
	p, ok := m.policies[id]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": "policy not found"})
		return
	}
	switch r.Method {
	case http.MethodPatch:
		for k, v := range decode(r) {
			if v != nil {
				p[k] = v
			}
		}
		m.policies[id] = p
		writeJSON(w, http.StatusOK, p)
	case http.MethodDelete:
		delete(m.policies, id)
		w.WriteHeader(http.StatusNoContent)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{})
	}
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
