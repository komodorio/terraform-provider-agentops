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
	{"/api/v1/accounts/members", "principal_id", "mbr", map[string]any{"status": "active", "display_name": "member", "user_id": "usr_1", "created_at": mockTS}},
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
	bindings    map[string]map[string]map[string]any // credential_id -> agent_id -> binding
	kbAgents    map[string]map[string]map[string]any // kb_id -> agent_id -> grant
	stores      map[string]map[string]map[string]any // generic CRUD: collection -> id -> record
	incidentPls map[string]map[string]any            // incident pipelines
	reviewWfs   map[string]map[string]any            // review workflows
	graderCfgs  map[string]map[string]any            // grader configs
	channels    map[string]map[string]any            // channels
	chanRoutes  map[string]map[string]map[string]any // channel_id -> route_id -> route
	hostedAgs   map[string]map[string]any            // "customer/agent_id" -> hosted agent
	seq         int
}

var (
	credBindingsRe = regexp.MustCompile(`^/api/v1/credentials/([^/]+)/bindings$`)
	credBindingRe  = regexp.MustCompile(`^/api/v1/credentials/([^/]+)/bindings/([^/]+)$`)
	kbAgentsRe     = regexp.MustCompile(`^/api/v1/knowledge-bases/([^/]+)/agents$`)
	kbAgentRe      = regexp.MustCompile(`^/api/v1/knowledge-bases/([^/]+)/agents/([^/]+)$`)
)

var (
	triggerIDRe  = regexp.MustCompile(`^/api/v1/triggers/([^/]+)$`)
	apiKeyIDRe   = regexp.MustCompile(`^/api/v1/api-keys/([^/]+)$`)
	scheduleRe   = regexp.MustCompile(`^/api/v1/schedules/([^/]+)$`)
	serviceAccRe = regexp.MustCompile(`^/api/v1/accounts/service-accounts/([^/]+)$`)
	policyRe     = regexp.MustCompile(`^/api/v1/gateway/admin/policies/([^/]+)$`)
)

var (
	incidentPipelineIDRe     = regexp.MustCompile(`^/api/v1/incident-pipelines/([^/]+)$`)
	incidentPipelineActionRe = regexp.MustCompile(`^/api/v1/incident-pipelines/([^/]+)/(activate|pause)$`)
	reviewWorkflowIDRe       = regexp.MustCompile(`^/api/v1/review-workflows/([^/]+)$`)
	reviewWorkflowActionRe   = regexp.MustCompile(`^/api/v1/review-workflows/([^/]+)/(activate|pause)$`)
)

var (
	graderConfigIDRe      = regexp.MustCompile(`^/api/v1/grader-configs/([^/]+)$`)
	channelIDRe           = regexp.MustCompile(`^/api/v1/channels/([^/]+)$`)
	channelActionRe       = regexp.MustCompile(`^/api/v1/channels/([^/]+)/(pause|resume)$`)
	channelRoutesRe       = regexp.MustCompile(`^/api/v1/channels/([^/]+)/routes$`)
	channelRouteIDRe      = regexp.MustCompile(`^/api/v1/channels/([^/]+)/routes/([^/]+)$`)
	hostedAgentByPathRe   = regexp.MustCompile(`^/api/v1/hosted-agents/([^/]+)/([^/]+)$`)
	workerCatalogDeployRe = regexp.MustCompile(`^/api/v1/worker-catalog/([^/]+)/deploy$`)
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
		bindings:    map[string]map[string]map[string]any{},
		kbAgents:    map[string]map[string]map[string]any{},
		stores:      map[string]map[string]map[string]any{},
		incidentPls: map[string]map[string]any{},
		reviewWfs:   map[string]map[string]any{},
		graderCfgs:  map[string]map[string]any{},
		channels:    map[string]map[string]any{},
		chanRoutes:  map[string]map[string]map[string]any{},
		hostedAgs:   map[string]map[string]any{},
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

	case credBindingsRe.MatchString(r.URL.Path):
		m.credBindings(w, r, credBindingsRe.FindStringSubmatch(r.URL.Path)[1])
	case credBindingRe.MatchString(r.URL.Path) && r.Method == http.MethodDelete:
		mm := credBindingRe.FindStringSubmatch(r.URL.Path)
		m.credBindingDelete(w, mm[1], mm[2])
	case kbAgentsRe.MatchString(r.URL.Path):
		m.kbAgentGrants(w, r, kbAgentsRe.FindStringSubmatch(r.URL.Path)[1])
	case kbAgentRe.MatchString(r.URL.Path) && r.Method == http.MethodDelete:
		mm := kbAgentRe.FindStringSubmatch(r.URL.Path)
		m.kbAgentDelete(w, mm[1], mm[2])

	case r.URL.Path == "/api/v1/integrations/catalog" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, []map[string]any{{"auth_config_key": "github", "auth_mode": "oauth", "category": "scm", "description": "GitHub", "name": "GitHub", "provider": "github", "available": true, "capabilities": []any{"repos.read"}}})
	case r.URL.Path == "/api/v1/authz/capabilities" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, []map[string]any{{"allows": "read", "domain": "agents", "key": "agent.invoke", "sensitivity": "low"}})
	case r.URL.Path == "/api/v1/authz/resource-types" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, []map[string]any{{"key": "agent", "notes": "an agent", "scope": "account"}})
	case r.URL.Path == "/api/v1/worker-catalog" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, []map[string]any{{"id": "wc_1", "name": "datadog-investigator", "description": "d", "category": "observability", "status": "available", "ready": true}})
	case r.URL.Path == "/api/v1/skills" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, []map[string]any{{"skill_id": "sk_1", "name": "search", "description": "d", "md5": "abc", "updated_at": mockTS, "tags": []any{"core"}}})
	case r.URL.Path == "/api/v1/reviewers" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, []map[string]any{{"agent_id": "agent_rev_1", "name": "security-reviewer", "description": "d", "is_builtin": true, "workflow_count": 0, "reviews_30d": 0, "findings_30d": 0}})

	case r.URL.Path == "/api/v1/incident-pipelines" && r.Method == http.MethodPost:
		m.createIncidentPipeline(w, r)
	case incidentPipelineActionRe.MatchString(r.URL.Path) && r.Method == http.MethodPost:
		mm := incidentPipelineActionRe.FindStringSubmatch(r.URL.Path)
		m.incidentPipelineAction(w, mm[1], mm[2])
	case incidentPipelineIDRe.MatchString(r.URL.Path):
		m.incidentPipelineByID(w, r, incidentPipelineIDRe.FindStringSubmatch(r.URL.Path)[1])

	case r.URL.Path == "/api/v1/review-workflows" && r.Method == http.MethodPost:
		m.createReviewWorkflow(w, r)
	case reviewWorkflowActionRe.MatchString(r.URL.Path) && r.Method == http.MethodPost:
		mm := reviewWorkflowActionRe.FindStringSubmatch(r.URL.Path)
		m.reviewWorkflowAction(w, mm[1], mm[2])
	case reviewWorkflowIDRe.MatchString(r.URL.Path):
		m.reviewWorkflowByID(w, r, reviewWorkflowIDRe.FindStringSubmatch(r.URL.Path)[1])

	case r.URL.Path == "/api/v1/grader-configs" && r.Method == http.MethodPost:
		m.createGraderConfig(w, r)
	case r.URL.Path == "/api/v1/grader-configs" && r.Method == http.MethodGet:
		m.listGraderConfigs(w, r)
	case graderConfigIDRe.MatchString(r.URL.Path):
		m.graderConfigByID(w, r, graderConfigIDRe.FindStringSubmatch(r.URL.Path)[1])

	case r.URL.Path == "/api/v1/channels" && r.Method == http.MethodPost:
		m.createChannel(w, r)
	case channelActionRe.MatchString(r.URL.Path) && r.Method == http.MethodPost:
		mm := channelActionRe.FindStringSubmatch(r.URL.Path)
		m.channelAction(w, mm[1], mm[2])
	case channelRoutesRe.MatchString(r.URL.Path):
		m.channelRoutes(w, r, channelRoutesRe.FindStringSubmatch(r.URL.Path)[1])
	case channelRouteIDRe.MatchString(r.URL.Path):
		mm := channelRouteIDRe.FindStringSubmatch(r.URL.Path)
		m.channelRouteByID(w, r, mm[1], mm[2])
	case channelIDRe.MatchString(r.URL.Path):
		m.channelByID(w, r, channelIDRe.FindStringSubmatch(r.URL.Path)[1])

	case r.URL.Path == "/api/v1/hosted-agents" && r.Method == http.MethodPost:
		m.createHostedAgent(w, r)
	case hostedAgentByPathRe.MatchString(r.URL.Path):
		mm := hostedAgentByPathRe.FindStringSubmatch(r.URL.Path)
		m.hostedAgentByPath(w, r, mm[1], mm[2])
	case workerCatalogDeployRe.MatchString(r.URL.Path) && r.Method == http.MethodPost:
		mm := workerCatalogDeployRe.FindStringSubmatch(r.URL.Path)
		m.deployWorkerCatalog(w, r, mm[1])

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

func (m *mockServer) credBindings(w http.ResponseWriter, r *http.Request, credID string) {
	switch r.Method {
	case http.MethodPost:
		body := decode(r)
		agent, _ := body["agent_id"].(string)
		rec := map[string]any{"agent_id": agent, "credential_id": credID, "created_at": mockTS}
		if v, ok := body["on_demand"]; ok && v != nil {
			rec["on_demand"] = v
		}
		if m.bindings[credID] == nil {
			m.bindings[credID] = map[string]map[string]any{}
		}
		m.bindings[credID][agent] = rec
		writeJSON(w, http.StatusCreated, rec)
	case http.MethodGet:
		out := make([]map[string]any, 0)
		for _, v := range m.bindings[credID] {
			out = append(out, v)
		}
		writeJSON(w, http.StatusOK, out)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{})
	}
}

func (m *mockServer) credBindingDelete(w http.ResponseWriter, credID, agentID string) {
	if m.bindings[credID] != nil {
		delete(m.bindings[credID], agentID)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (m *mockServer) kbAgentGrants(w http.ResponseWriter, r *http.Request, kbID string) {
	switch r.Method {
	case http.MethodPost:
		body := decode(r)
		agent, _ := body["agent_id"].(string)
		rec := map[string]any{"agent_id": agent, "kb_id": kbID, "grant_id": m.nextID("grant"), "created_at": mockTS}
		if m.kbAgents[kbID] == nil {
			m.kbAgents[kbID] = map[string]map[string]any{}
		}
		m.kbAgents[kbID][agent] = rec
		writeJSON(w, http.StatusCreated, rec)
	case http.MethodGet:
		out := make([]map[string]any, 0)
		for _, v := range m.kbAgents[kbID] {
			out = append(out, v)
		}
		writeJSON(w, http.StatusOK, out)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{})
	}
}

func (m *mockServer) kbAgentDelete(w http.ResponseWriter, kbID, agentID string) {
	if m.kbAgents[kbID] != nil {
		delete(m.kbAgents[kbID], agentID)
	}
	w.WriteHeader(http.StatusNoContent)
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

// incidentPipelineDerive fills the computed/derived fields the detail response
// always carries, from the stored request-shaped record.
func incidentPipelineDerive(rec map[string]any) {
	alert, _ := rec["alert_source"].(map[string]any)
	if alert != nil {
		rec["source_provider"] = alert["provider"]
	}
	binding, _ := rec["orchestrator_binding"].(map[string]any)
	if binding == nil {
		binding = map[string]any{"agent_id": "agent_orch_default"}
		rec["orchestrator_binding"] = binding
	}
	rec["orchestrator_agent_id"] = binding["agent_id"]

	rule, _ := rec["routing_rule"].(map[string]any)
	if rule == nil {
		rec["routing_rule"] = map[string]any{"route_all": true, "missing_field_default": false}
	} else {
		if rule["route_all"] == nil {
			rule["route_all"] = false
		}
		if rule["missing_field_default"] == nil {
			rule["missing_field_default"] = false
		}
	}

	specialists, _ := rec["specialist_bindings"].([]any)
	rec["specialist_count"] = len(specialists)

	if dc, ok := rec["delivery_config"].(map[string]any); ok {
		if slack, ok := dc["slack"].(map[string]any); ok {
			if slack["channel_name"] == nil {
				slack["channel_name"] = "chan-" + toString(slack["channel_id"])
			}
			if slack["enabled"] == nil {
				slack["enabled"] = true
			}
		}
	}
}

func (m *mockServer) createIncidentPipeline(w http.ResponseWriter, r *http.Request) {
	body := decode(r)
	id := m.nextID("ipl")
	rec := cloneMap(body)
	rec["id"] = id
	rec["status"] = "draft"
	rec["created_at"] = mockTS
	rec["webhook_url"] = "https://mock.local/webhooks/" + id
	rec["webhook_token"] = "wht_" + id
	if rec["name"] == nil {
		rec["name"] = "pipeline-" + id
	}
	incidentPipelineDerive(rec)
	m.incidentPls[id] = rec
	writeJSON(w, http.StatusCreated, rec)
}

func (m *mockServer) incidentPipelineByID(w http.ResponseWriter, r *http.Request, id string) {
	rec, ok := m.incidentPls[id]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": "incident pipeline not found"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, rec)
	case http.MethodPatch:
		// The real API only allows updating a pipeline while it is `draft`.
		if status, _ := rec["status"].(string); status != "draft" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"detail": "Only draft pipelines can be updated. Pause the pipeline first."})
			return
		}
		for k, v := range decode(r) {
			if v != nil {
				rec[k] = v
			}
		}
		incidentPipelineDerive(rec)
		m.incidentPls[id] = rec
		writeJSON(w, http.StatusOK, rec)
	case http.MethodDelete:
		// The real API refuses to delete an active pipeline; it must be paused first.
		if status, _ := rec["status"].(string); status == "active" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"detail": "Pause the pipeline before deleting it."})
			return
		}
		delete(m.incidentPls, id)
		w.WriteHeader(http.StatusNoContent)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{})
	}
}

func (m *mockServer) incidentPipelineAction(w http.ResponseWriter, id, action string) {
	rec, ok := m.incidentPls[id]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": "incident pipeline not found"})
		return
	}
	if action == "activate" {
		// The real API refuses to activate a pipeline with no linked endpoint.
		if tid, _ := rec["trigger_id"].(string); tid == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"detail": "Link an endpoint before activating. Pick one in the workflow wizard."})
			return
		}
		rec["status"] = "active"
	} else {
		rec["status"] = "paused"
	}
	m.incidentPls[id] = rec
	writeJSON(w, http.StatusOK, rec)
}

// reviewWorkflowDerive fills the computed fields the detail response carries.
func reviewWorkflowDerive(rec map[string]any) {
	repos, _ := rec["repos"].([]any)
	rec["repo_count"] = len(repos)
	status := make([]any, 0, len(repos))
	for i, ri := range repos {
		repo, _ := ri.(map[string]any)
		status = append(status, map[string]any{
			"repo_owner":     repo["repo_owner"],
			"repo_name":      repo["repo_name"],
			"webhook_status": "active",
			"github_hook_id": 1000 + i,
		})
	}
	rec["repos"] = status
	if rec["reviewer_agent_ids"] == nil {
		rec["reviewer_agent_ids"] = []any{}
	}
}

func (m *mockServer) createReviewWorkflow(w http.ResponseWriter, r *http.Request) {
	body := decode(r)
	id := m.nextID("rvw")
	rec := cloneMap(body)
	rec["id"] = id
	rec["status"] = "draft"
	rec["created_at"] = mockTS
	rec["updated_at"] = mockTS
	rec["webhook_url"] = "https://mock.local/review/" + id
	if rec["name"] == nil {
		rec["name"] = "review-" + id
	}
	reviewWorkflowDerive(rec)
	m.reviewWfs[id] = rec
	writeJSON(w, http.StatusCreated, rec)
}

func (m *mockServer) reviewWorkflowByID(w http.ResponseWriter, r *http.Request, id string) {
	rec, ok := m.reviewWfs[id]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": "review workflow not found"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, rec)
	case http.MethodPatch:
		for k, v := range decode(r) {
			if v != nil {
				rec[k] = v
			}
		}
		reviewWorkflowDerive(rec)
		rec["updated_at"] = mockTS
		m.reviewWfs[id] = rec
		writeJSON(w, http.StatusOK, rec)
	case http.MethodDelete:
		delete(m.reviewWfs, id)
		w.WriteHeader(http.StatusNoContent)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{})
	}
}

func (m *mockServer) reviewWorkflowAction(w http.ResponseWriter, id, action string) {
	rec, ok := m.reviewWfs[id]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": "review workflow not found"})
		return
	}
	if action == "activate" {
		rec["status"] = "active"
	} else {
		rec["status"] = "paused"
	}
	m.reviewWfs[id] = rec
	writeJSON(w, http.StatusOK, rec)
}

func (m *mockServer) createGraderConfig(w http.ResponseWriter, r *http.Request) {
	body := decode(r)
	id := m.nextID("grd")
	rec := cloneMap(body)
	rec["id"] = id
	rec["runs_seen"] = 0
	rec["created_at"] = mockTS
	rec["updated_at"] = mockTS
	if rec["sample_rate"] == nil {
		rec["sample_rate"] = 100
	}
	if rec["guidelines"] == nil {
		rec["guidelines"] = ""
	}
	m.graderCfgs[id] = rec
	writeJSON(w, http.StatusCreated, rec)
}

func (m *mockServer) listGraderConfigs(w http.ResponseWriter, r *http.Request) {
	target := r.URL.Query().Get("target_agent_id")
	out := make([]map[string]any, 0)
	for _, v := range m.graderCfgs {
		if target != "" && toString(v["target_agent_id"]) != target {
			continue
		}
		out = append(out, v)
	}
	writeJSON(w, http.StatusOK, map[string]any{"configs": out})
}

func (m *mockServer) graderConfigByID(w http.ResponseWriter, r *http.Request, id string) {
	rec, ok := m.graderCfgs[id]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": "grader config not found"})
		return
	}
	switch r.Method {
	case http.MethodPatch:
		for k, v := range decode(r) {
			if v != nil {
				rec[k] = v
			}
		}
		rec["updated_at"] = mockTS
		m.graderCfgs[id] = rec
		writeJSON(w, http.StatusOK, rec)
	case http.MethodDelete:
		delete(m.graderCfgs, id)
		w.WriteHeader(http.StatusNoContent)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{})
	}
}

func (m *mockServer) createChannel(w http.ResponseWriter, r *http.Request) {
	body := decode(r)
	id := m.nextID("chn")
	rec := map[string]any{
		"id":           id,
		"account_id":   "acct_mock",
		"provider":     body["provider"],
		"connector":    body["connector"],
		"display_name": body["display_name"],
		"slug":         "slug-" + id,
		"status":       "active",
		"created_at":   mockTS,
		"updated_at":   mockTS,
	}
	if v := body["config"]; v != nil {
		rec["config_json"] = v
	}
	for _, k := range []string{"labels", "external_id", "integration_id"} {
		if v := body[k]; v != nil {
			rec[k] = v
		}
	}
	m.channels[id] = rec
	writeJSON(w, http.StatusCreated, rec)
}

func (m *mockServer) channelByID(w http.ResponseWriter, r *http.Request, id string) {
	rec, ok := m.channels[id]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": "channel not found"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, rec)
	case http.MethodPatch:
		for k, v := range decode(r) {
			if v == nil || k == "app_token" {
				continue
			}
			if k == "config" {
				rec["config_json"] = v
				continue
			}
			rec[k] = v
		}
		rec["updated_at"] = mockTS
		m.channels[id] = rec
		writeJSON(w, http.StatusOK, rec)
	case http.MethodDelete:
		delete(m.channels, id)
		delete(m.chanRoutes, id)
		w.WriteHeader(http.StatusNoContent)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{})
	}
}

func (m *mockServer) channelAction(w http.ResponseWriter, id, action string) {
	rec, ok := m.channels[id]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": "channel not found"})
		return
	}
	if action == "pause" {
		rec["status"] = "paused"
	} else {
		rec["status"] = "active"
	}
	m.channels[id] = rec
	writeJSON(w, http.StatusOK, rec)
}

func (m *mockServer) channelRoutes(w http.ResponseWriter, r *http.Request, channelID string) {
	switch r.Method {
	case http.MethodPost:
		body := decode(r)
		rid := m.nextID("rt")
		rec := map[string]any{
			"id":          rid,
			"account_id":  "acct_mock",
			"channel_id":  channelID,
			"rule_type":   body["rule_type"],
			"target_type": body["target_type"],
			"target_id":   body["target_id"],
			"priority":    body["priority"],
			"is_default":  valueOr(body["is_default"], false),
			"is_enabled":  valueOr(body["is_enabled"], true),
			"created_at":  mockTS,
			"updated_at":  mockTS,
		}
		if v := body["match"]; v != nil {
			rec["match_json"] = v
		}
		if v := body["input"]; v != nil {
			rec["input_json"] = v
		}
		if m.chanRoutes[channelID] == nil {
			m.chanRoutes[channelID] = map[string]map[string]any{}
		}
		m.chanRoutes[channelID][rid] = rec
		writeJSON(w, http.StatusCreated, rec)
	case http.MethodGet:
		out := make([]map[string]any, 0)
		for _, v := range m.chanRoutes[channelID] {
			out = append(out, v)
		}
		writeJSON(w, http.StatusOK, out)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{})
	}
}

func (m *mockServer) channelRouteByID(w http.ResponseWriter, r *http.Request, channelID, routeID string) {
	routes := m.chanRoutes[channelID]
	rec, ok := routes[routeID]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": "route not found"})
		return
	}
	switch r.Method {
	case http.MethodPatch:
		for k, v := range decode(r) {
			if v == nil {
				continue
			}
			switch k {
			case "match":
				rec["match_json"] = v
			case "input":
				rec["input_json"] = v
			default:
				rec[k] = v
			}
		}
		rec["updated_at"] = mockTS
		routes[routeID] = rec
		writeJSON(w, http.StatusOK, rec)
	case http.MethodDelete:
		delete(routes, routeID)
		w.WriteHeader(http.StatusNoContent)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{})
	}
}

func (m *mockServer) createHostedAgent(w http.ResponseWriter, r *http.Request) {
	body := decode(r)
	customer := toString(body["customer"])
	if customer == "" {
		customer = "acme" // server derives the customer from the account when omitted
	}
	agentID := toString(body["agentId"])
	id := m.nextID("ha")
	rec := map[string]any{
		"id":             id,
		"customer":       customer,
		"agentId":        agentID,
		"identity":       "identity-" + id,
		"runtimeAgentId": "rt-" + id,
		"repoOwner":      "komodorio",
		"repoName":       "agent-" + agentID,
		"repoBranch":     "main",
		"repoPath":       "/",
		"status":         "deploying",
		"createdAt":      mockTS,
		"updatedAt":      mockTS,
	}
	m.hostedAgs[customer+"/"+agentID] = rec
	writeJSON(w, http.StatusCreated, rec)
}

// deployWorkerCatalog simulates a catalog deploy: the server derives the customer
// and assigns an agent_id when the client omits one, then returns a hosted agent
// (which the resource then reads/deletes via the hosted-agents endpoints).
func (m *mockServer) deployWorkerCatalog(w http.ResponseWriter, r *http.Request, catalogID string) {
	body := decode(r)
	agentID := toString(body["agentId"])
	if agentID == "" {
		agentID = "deployed-" + catalogID
	}
	customer := "acme" // server-derived from the account
	id := m.nextID("ha")
	rec := map[string]any{
		"id":             id,
		"customer":       customer,
		"agentId":        agentID,
		"identity":       "identity-" + id,
		"runtimeAgentId": "rt-" + id,
		"repoOwner":      "komodorio",
		"repoName":       "agent-" + agentID,
		"repoBranch":     "main",
		"repoPath":       "/",
		"status":         "deploying",
		"createdAt":      mockTS,
		"updatedAt":      mockTS,
	}
	m.hostedAgs[customer+"/"+agentID] = rec
	writeJSON(w, http.StatusCreated, rec)
}

func (m *mockServer) hostedAgentByPath(w http.ResponseWriter, r *http.Request, customer, agentID string) {
	key := customer + "/" + agentID
	rec, ok := m.hostedAgs[key]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": "hosted agent not found"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		// Simulate provisioning completing: an agent created as "deploying"
		// reports "online" once polled, so wait_for_online terminates.
		if rec["status"] == "deploying" {
			rec["status"] = "online"
			m.hostedAgs[key] = rec
		}
		writeJSON(w, http.StatusOK, rec)
	case http.MethodPut:
		_ = decode(r) // spec fields are not echoed back
		rec["updatedAt"] = mockTS
		rec["status"] = "online"
		m.hostedAgs[key] = rec
		writeJSON(w, http.StatusOK, rec)
	case http.MethodDelete:
		delete(m.hostedAgs, key)
		w.WriteHeader(http.StatusNoContent)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{})
	}
}

func valueOr(v, def any) any {
	if v == nil {
		return def
	}
	return v
}

func toString(v any) string {
	s, _ := v.(string)
	return s
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
