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
	channels    map[string]map[string]any            // channels
	chanRoutes  map[string]map[string]map[string]any // channel_id -> route_id -> route
	hostedAgs   map[string]map[string]any            // "customer/agent_id" -> hosted agent
	outposts    map[string]map[string]any            // outposts
	// selfHostedAgs are the agents a self-hosted catalog deploy registers, keyed by
	// agent id. They live under /api/v1/agents, not /api/v1/hosted-agents.
	selfHostedAgs map[string]map[string]any
	// selfHostedTriggerFails makes the self-hosted deploy answer 207 with one failed
	// trigger, the partial success the resource must keep rather than roll back.
	selfHostedTriggerFails bool
	// selfHostedMcpGroupFails makes the self-hosted deploy answer 207 with a failed
	// mcp_group_id settings section and no group bound. The real endpoint applies
	// settings after the token is minted and reports per-field failure instead of
	// raising, so the post-deploy read can honestly report no group at all.
	selfHostedMcpGroupFails bool
	// agentReadFails is how many GET /agents/{id} reads are refused before the
	// record is served again, standing in for a read that fails right after a
	// mutation. 403 rather than a 5xx because the client retries 5xx away.
	agentReadFails int
	// mcpGroupBindFails refuses PATCH /agents/{id}/mcp-group, the follow-up call a
	// create makes after the agent is already registered.
	mcpGroupBindFails bool
	// mcpGroupBindSilent accepts PATCH /agents/{id}/mcp-group without recording it,
	// so the agent read that follows does not echo the group back. It stands in for
	// a bind the control plane took but the read has not caught up with.
	mcpGroupBindSilent bool
	// serverUpdateDropsOutpost makes PATCH /gateway/admin/servers/{id} answer without
	// an outpost_id. The real UpdateServerRequest has no outpost field, so its
	// response can echo null for a server that is bound.
	serverUpdateDropsOutpost bool
	// deployFails makes every hosted agent this server creates behave like a failed
	// cluster-side provision: the hosted record stays "draft" (that record's status
	// only ever tracks heartbeats) while the runtime agent record reports the failure.
	deployFails bool
	// runtimeNotFound is how many runtime-agent reads 404 before the record appears,
	// standing in for the runtime record lagging the hosted one after create.
	runtimeNotFound int
	// hostedPollsDeploying is how many GETs a hosted agent stays "deploying" before
	// it comes online. Zero, the default, comes online on the first read.
	hostedPollsDeploying int
	// deleteNeedsArchive mirrors an account with hosted lifecycle enabled, where a
	// successfully deployed agent cannot be deleted until it has been archived.
	deleteNeedsArchive bool
	// archiveSettlesAfter is how many deploy-status reads the archive stays
	// in_progress for, standing in for the scale-to-zero deploy taking a while.
	archiveSettlesAfter int
	// deleteAgentAnswer overrides how DELETE /agents/{id} answers; see the
	// deleteAgentMode constants. Empty is the normal 204.
	deleteAgentAnswer deleteAgentMode
	// heartbeatedAgs are the self-hosted agents whose worker has come online at
	// least once. The draft delete — the only one not behind the lifecycle feature
	// — refuses them, so this is what separates an agent Terraform can still tear
	// down on a gate-off account from one it cannot.
	heartbeatedAgs map[string]bool
	// deletedAgs are the self-hosted agents a delete removed. Without it an unknown
	// id falls through to the synthesized hosted-runtime record below and reads 200
	// forever, so a deleted agent would still look alive.
	deletedAgs map[string]bool
	// revokedTokens records the agents whose worker token the delete revoked. The
	// real API only revokes on the decommission path, which a delete issued while
	// the archive deploy is still in flight never reaches.
	revokedTokens map[string]bool
	seq           int
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
	channelIDRe           = regexp.MustCompile(`^/api/v1/channels/([^/]+)$`)
	channelActionRe       = regexp.MustCompile(`^/api/v1/channels/([^/]+)/(pause|resume)$`)
	channelRoutesRe       = regexp.MustCompile(`^/api/v1/channels/([^/]+)/routes$`)
	channelRouteIDRe      = regexp.MustCompile(`^/api/v1/channels/([^/]+)/routes/([^/]+)$`)
	hostedAgentByPathRe   = regexp.MustCompile(`^/api/v1/hosted-agents/([^/]+)/([^/]+)$`)
	runtimeAgentByIDRe    = regexp.MustCompile(`^/api/v1/agents/([^/]+)$`)
	hostedAgentArchiveRe  = regexp.MustCompile(`^/api/v1/hosted-agents/([^/]+)/([^/]+)/archive$`)
	workerCatalogDeployRe = regexp.MustCompile(`^/api/v1/worker-catalog/([^/]+)/deploy$`)

	agentMcpGroupRe = regexp.MustCompile(`^/api/v1/agents/([^/]+)/mcp-group$`)
	agentDraftRe    = regexp.MustCompile(`^/api/v1/agents/([^/]+)/draft$`)

	outpostIDRe      = regexp.MustCompile(`^/api/v1/outposts/([^/]+)$`)
	outpostInstallRe = regexp.MustCompile(`^/api/v1/outposts/([^/]+)/install$`)
	serverOutpostRe  = regexp.MustCompile(`^/api/v1/gateway/admin/servers/([^/]+)/outpost$`)
	serverIDRe       = regexp.MustCompile(`^/api/v1/gateway/admin/servers/([^/]+)$`)

	runtimeAgentArchiveRe         = regexp.MustCompile(`^/api/v1/agents/([^/]+)/archive$`)
	workerCatalogSelfHostedDeploy = regexp.MustCompile(`^/api/v1/worker-catalog/([^/]+)/self-hosted-deploy$`)
)

// tune applies mock configuration under the lock every handler reads it with, so a
// test can set up a server that is already serving traffic.
func (m *mockServer) tune(fn func(*mockServer)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	fn(m)
}

func newMockServer(t *testing.T) *mockServer {
	t.Helper()
	m := &mockServer{
		triggers:       map[string]map[string]any{},
		apiKeys:        map[string]map[string]any{},
		schedules:      map[string]map[string]any{},
		serviceAccs:    map[string]map[string]any{},
		policies:       map[string]map[string]any{},
		conns:          map[string]map[string]any{},
		bindings:       map[string]map[string]map[string]any{},
		kbAgents:       map[string]map[string]map[string]any{},
		stores:         map[string]map[string]map[string]any{},
		incidentPls:    map[string]map[string]any{},
		reviewWfs:      map[string]map[string]any{},
		channels:       map[string]map[string]any{},
		chanRoutes:     map[string]map[string]map[string]any{},
		hostedAgs:      map[string]map[string]any{},
		outposts:       map[string]map[string]any{},
		revokedTokens:  map[string]bool{},
		deletedAgs:     map[string]bool{},
		heartbeatedAgs: map[string]bool{},
		selfHostedAgs:  map[string]map[string]any{},
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

	case r.URL.Path == "/api/v1/outposts" && r.Method == http.MethodPost:
		m.createOutpost(w, r)
	case r.URL.Path == "/api/v1/outposts" && r.Method == http.MethodGet:
		m.listMaps(w, m.outposts)
	case outpostInstallRe.MatchString(r.URL.Path) && r.Method == http.MethodGet:
		m.outpostInstall(w, outpostInstallRe.FindStringSubmatch(r.URL.Path)[1])
	case outpostIDRe.MatchString(r.URL.Path):
		m.outpostByID(w, r, outpostIDRe.FindStringSubmatch(r.URL.Path)[1])
	case serverOutpostRe.MatchString(r.URL.Path):
		m.serverOutpost(w, r, serverOutpostRe.FindStringSubmatch(r.URL.Path)[1])
	case serverIDRe.MatchString(r.URL.Path) && r.Method == http.MethodPatch && m.serverUpdateDropsOutpost:
		m.updateServerWithoutOutpost(w, r, serverIDRe.FindStringSubmatch(r.URL.Path)[1])

	case r.URL.Path == "/api/v1/agents" && r.Method == http.MethodPost:
		m.createRuntimeAgent(w, r)
	case agentMcpGroupRe.MatchString(r.URL.Path) && r.Method == http.MethodPatch:
		m.setAgentMcpGroup(w, r, agentMcpGroupRe.FindStringSubmatch(r.URL.Path)[1])
	case agentDraftRe.MatchString(r.URL.Path) && r.Method == http.MethodDelete:
		m.deleteRuntimeAgentDraft(w, agentDraftRe.FindStringSubmatch(r.URL.Path)[1])

	case r.URL.Path == "/api/v1/hosted-agents" && r.Method == http.MethodPost:
		m.createHostedAgent(w, r)
	case hostedAgentArchiveRe.MatchString(r.URL.Path) && r.Method == http.MethodPost:
		mm := hostedAgentArchiveRe.FindStringSubmatch(r.URL.Path)
		m.archiveHostedAgent(w, mm[1], mm[2])
	case hostedAgentByPathRe.MatchString(r.URL.Path):
		mm := hostedAgentByPathRe.FindStringSubmatch(r.URL.Path)
		m.hostedAgentByPath(w, r, mm[1], mm[2])
	case runtimeAgentArchiveRe.MatchString(r.URL.Path) && r.Method == http.MethodPatch:
		m.archiveRuntimeAgent(w, runtimeAgentArchiveRe.FindStringSubmatch(r.URL.Path)[1])
	case runtimeAgentByIDRe.MatchString(r.URL.Path) && r.Method == http.MethodGet:
		m.runtimeAgentByID(w, runtimeAgentByIDRe.FindStringSubmatch(r.URL.Path)[1])
	case runtimeAgentByIDRe.MatchString(r.URL.Path) && r.Method == http.MethodDelete:
		m.deleteRuntimeAgent(w, runtimeAgentByIDRe.FindStringSubmatch(r.URL.Path)[1])
	case workerCatalogDeployRe.MatchString(r.URL.Path) && r.Method == http.MethodPost:
		mm := workerCatalogDeployRe.FindStringSubmatch(r.URL.Path)
		m.deployWorkerCatalog(w, r, mm[1])
	case workerCatalogSelfHostedDeploy.MatchString(r.URL.Path) && r.Method == http.MethodPost:
		mm := workerCatalogSelfHostedDeploy.FindStringSubmatch(r.URL.Path)
		m.selfHostedDeployWorkerCatalog(w, r, mm[1])

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
		// The real API only allows updating a workflow while it is `draft`.
		if status, _ := rec["status"].(string); status != "draft" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"detail": "Only draft workflows can be updated. Pause the workflow first."})
			return
		}
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
		// The real API refuses to delete an active workflow; it must be paused first.
		if status, _ := rec["status"].(string); status == "active" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"detail": "Pause the workflow before deleting it."})
			return
		}
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

// createOutpost answers with the outpost, its one-time credential and the install
// values, the way POST /outposts does.
func (m *mockServer) createOutpost(w http.ResponseWriter, r *http.Request) {
	body := decode(r)
	id := m.nextID("op")
	rec := map[string]any{
		"outpost_id": id,
		"name":       body["name"],
		"status":     "pending",
		"connected":  false,
		"created_at": mockTS,
		"updated_at": mockTS,
		"allowlist":  valueOr(body["allowlist"], []any{}),
		"labels":     valueOr(body["labels"], map[string]any{}),
	}
	if v, ok := body["description"]; ok && v != nil {
		rec["description"] = v
	}
	m.outposts[id] = rec
	writeJSON(w, http.StatusCreated, map[string]any{
		"outpost":    rec,
		"credential": map[string]any{"credential": "opc_" + id + "_secret", "hint": "opc_..."},
		"install":    outpostInstallFor(id),
	})
}

func (m *mockServer) outpostByID(w http.ResponseWriter, r *http.Request, id string) {
	rec, ok := m.outposts[id]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": "outpost not found"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, rec)
	case http.MethodPatch:
		// A partial update: an omitted field is left alone, and a supplied
		// allowlist/labels replaces wholesale. Clearing needs an explicit empty.
		for k, v := range decode(r) {
			if v != nil {
				rec[k] = v
			}
		}
		rec["updated_at"] = mockTS
		m.outposts[id] = rec
		writeJSON(w, http.StatusOK, rec)
	case http.MethodDelete:
		delete(m.outposts, id)
		w.WriteHeader(http.StatusNoContent)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{})
	}
}

func (m *mockServer) outpostInstall(w http.ResponseWriter, id string) {
	if _, ok := m.outposts[id]; !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": "outpost not found"})
		return
	}
	writeJSON(w, http.StatusOK, outpostInstallFor(id))
}

func outpostInstallFor(id string) map[string]any {
	return map[string]any{
		"chart":                  "oci://ghcr.io/komodorio/charts/agentops-outpost",
		"command":                "helm install " + id + " agentops-outpost",
		"upgrade_command":        "helm upgrade " + id + " agentops-outpost",
		"values":                 "outpostId: " + id + "\n",
		"upgrade_values":         "outpostId: " + id + "\n",
		"namespace":              "agentops",
		"release_name":           id,
		"credential_secret_name": id + "-credential",
		"credential_secret_key":  "credential",
	}
}

// serverOutpost binds or unbinds an upstream's egress. The binding lives on the
// server record, which is what the server read echoes back as outpost_id.
func (m *mockServer) serverOutpost(w http.ResponseWriter, r *http.Request, serverID string) {
	rec, ok := m.stores["/api/v1/gateway/admin/servers"][serverID]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": "server not found"})
		return
	}
	switch r.Method {
	case http.MethodPut:
		outpostID := toString(decode(r)["outpost_id"])
		if _, known := m.outposts[outpostID]; !known {
			writeJSON(w, http.StatusNotFound, map[string]any{"detail": "outpost not found"})
			return
		}
		rec["outpost_id"] = outpostID
		writeJSON(w, http.StatusOK, map[string]any{"server_id": serverID, "outpost_id": outpostID})
	case http.MethodDelete:
		delete(rec, "outpost_id")
		w.WriteHeader(http.StatusNoContent)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{})
	}
}

// updateServerWithoutOutpost patches a server and answers without outpost_id, the
// shape the real update takes because UpdateServerRequest has no outpost field.
func (m *mockServer) updateServerWithoutOutpost(w http.ResponseWriter, r *http.Request, serverID string) {
	rec, ok := m.stores["/api/v1/gateway/admin/servers"][serverID]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": "server not found"})
		return
	}
	for k, v := range decode(r) {
		if v != nil {
			rec[k] = v
		}
	}
	echo := cloneMap(rec)
	delete(echo, "outpost_id")
	writeJSON(w, http.StatusOK, echo)
}

// serverOutpostBinding reports the outpost an upstream is bound to, for assertions
// the Terraform state cannot make on its own.
func (m *mockServer) serverOutpostBinding(serverID string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return toString(m.stores["/api/v1/gateway/admin/servers"][serverID]["outpost_id"])
}

// outpostRecord returns a copy of a stored outpost, for assertions on what an
// update actually wrote.
func (m *mockServer) outpostRecord(id string) map[string]any {
	m.mu.Lock()
	defer m.mu.Unlock()
	return cloneMap(m.outposts[id])
}

// unbindAgentMcpGroup drops an agent's MCP group behind Terraform's back, the way
// someone unbinding it in the UI or by a direct API call would.
func (m *mockServer) unbindAgentMcpGroup(agentID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if rec, ok := m.selfHostedAgs[agentID]; ok {
		delete(rec, "mcp_group_id")
	}
}

// bindAgentMcpGroup binds a group behind Terraform's back, standing in for a
// binding the resource does not own — a catalog entry's own group, or one someone
// attached in the UI.
func (m *mockServer) bindAgentMcpGroup(agentID, groupID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if rec, ok := m.selfHostedAgs[agentID]; ok {
		rec["mcp_group_id"] = groupID
	}
}

// agentMcpGroup reports the group actually bound on the control plane, so a test
// can tell a converged apply from state that merely claims to be converged.
func (m *mockServer) agentMcpGroup(agentID string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return toString(m.selfHostedAgs[agentID]["mcp_group_id"])
}

// createRuntimeAgent registers a self-hosted agent and renders its Helm values,
// the way POST /agents does.
func (m *mockServer) createRuntimeAgent(w http.ResponseWriter, r *http.Request) {
	body := decode(r)
	slug := toString(body["agentId"])
	opaqueID := m.nextID("ag")
	name := toString(body["displayName"])
	if name == "" {
		name = slug
	}
	agent := map[string]any{
		"agent_id": opaqueID, "id_slug": slug, "status": "draft", "name": name,
		"created_at": mockTS, "is_archived": false,
	}
	m.selfHostedAgs[opaqueID] = agent
	m.selfHostedAgs[slug] = agent
	writeJSON(w, http.StatusOK, map[string]any{
		"agentId":         opaqueID,
		"values":          "workerToken: wt_" + slug + "_secret\n",
		"command":         "helm install " + slug + " agentops-agent-base",
		"workerTokenHint": "wt_" + firstN(slug, 4) + "...",
	})
}

func (m *mockServer) setAgentMcpGroup(w http.ResponseWriter, r *http.Request, agentID string) {
	if m.mcpGroupBindFails {
		writeJSON(w, http.StatusForbidden, map[string]any{"detail": "mcp group binding refused"})
		return
	}
	rec, ok := m.selfHostedAgs[agentID]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": "agent not found"})
		return
	}
	// The control plane coerces the request through `mcp_group_id or None`, so an
	// empty id is an unbind and every later read reports null rather than "".
	if v := decode(r)["mcp_group_id"]; v != nil && v != "" && !m.mcpGroupBindSilent {
		rec["mcp_group_id"] = v
	} else if v == nil || v == "" {
		delete(rec, "mcp_group_id")
	}
	writeJSON(w, http.StatusOK, rec)
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
		"status":         m.initialHostedStatus(),
		"deployStatus":   "done",
		"createdAt":      mockTS,
		"updatedAt":      mockTS,
	}
	m.hostedAgs[customer+"/"+agentID] = rec
	writeJSON(w, http.StatusCreated, rec)
}

// initialHostedStatus is "deploying" for a provision that will come online, and
// "draft" for one that fails: a hosted record whose worker never heartbeats never
// leaves draft, which is exactly what hides the failure from that endpoint.
func (m *mockServer) initialHostedStatus() string {
	if m.deployFails {
		return "draft"
	}
	return "deploying"
}

// runtimeAgentByID serves the runtime agent record, the only one carrying the
// deploy outcome.
func (m *mockServer) runtimeAgentByID(w http.ResponseWriter, agentID string) {
	if m.agentReadFails > 0 {
		m.agentReadFails--
		writeJSON(w, http.StatusForbidden, map[string]any{"detail": "read refused"})
		return
	}
	if m.runtimeNotFound > 0 {
		m.runtimeNotFound--
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": "agent not found"})
		return
	}
	if rec, ok := m.selfHostedAgs[agentID]; ok {
		writeJSON(w, http.StatusOK, rec)
		return
	}
	if m.deletedAgs[agentID] {
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": "agent not found"})
		return
	}
	rec := map[string]any{"agent_id": agentID, "status": "draft", "deploy_status": m.deployStatusFor(agentID)}
	if m.deployFails {
		rec["status"] = "deploy_failed"
		rec["deploy_status"] = "failed"
		rec["deploy_error"] = "Deploy failed — could not provision the agent. Please try again or contact support."
	}
	writeJSON(w, http.StatusOK, rec)
}

// deployStatusFor reports the deploy status of the hosted agent registered under
// this runtime id, letting an archive settle after archiveSettlesAfter reads.
func (m *mockServer) deployStatusFor(runtimeAgentID string) string {
	for key, rec := range m.hostedAgs {
		if toString(rec["runtimeAgentId"]) != runtimeAgentID {
			continue
		}
		if rec["deployStatus"] == "in_progress" && m.archiveSettlesAfter > 0 {
			m.archiveSettlesAfter--
			return "in_progress"
		}
		rec["deployStatus"] = "done"
		m.hostedAgs[key] = rec
		return "done"
	}
	return "done"
}

// archiveHostedAgent marks a hosted agent archived, the step the control plane
// demands before a live agent may be deleted.
func (m *mockServer) archiveHostedAgent(w http.ResponseWriter, customer, agentID string) {
	key := customer + "/" + agentID
	rec, ok := m.hostedAgs[key]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": "hosted agent not found"})
		return
	}
	rec["isArchived"] = true
	rec["deployStatus"] = "in_progress"
	writeJSON(w, http.StatusOK, rec)
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
		"status":         m.initialHostedStatus(),
		"deployStatus":   "done",
		"createdAt":      mockTS,
		"updatedAt":      mockTS,
	}
	m.hostedAgs[customer+"/"+agentID] = rec
	writeJSON(w, http.StatusCreated, rec)
}

// selfHostedDeployWorkerCatalog mints a worker token for a catalog entry the
// operator will run themselves. With selfHostedTriggerFails or
// selfHostedMcpGroupFails set it answers 207, the partial success the real
// endpoint returns when the agent was created but something it asked for was not.
// Both may be set at once, which is what the real endpoint does when a settings
// field and a trigger both fail.
func (m *mockServer) selfHostedDeployWorkerCatalog(w http.ResponseWriter, r *http.Request, catalogID string) {
	body := decode(r)
	// The friendly slug defaults to the catalog entry's own id, and the response
	// carries only the opaque id — never the slug. Both address the agent.
	slug := toString(body["agentId"])
	if slug == "" {
		slug = catalogID
	}
	opaqueID := m.nextID("ag")
	agent := map[string]any{
		"agent_id": opaqueID, "id_slug": slug, "status": "draft", "name": slug, "created_at": mockTS,
	}
	// The deploy request's mcpGroupId is folded into the agent settings and applied
	// onto the agent row, so GET /agents/{id} echoes it from here on.
	if v := body["mcpGroupId"]; v != nil && !m.selfHostedMcpGroupFails {
		agent["mcp_group_id"] = v
	}
	m.selfHostedAgs[opaqueID] = agent
	m.selfHostedAgs[slug] = agent

	rec := map[string]any{
		"agentId":         opaqueID,
		"token":           "wt_" + slug + "_secret",
		"workerTokenHint": "wt_" + firstN(slug, 4) + "...",
	}
	if m.selfHostedMcpGroupFails {
		rec["settings"] = map[string]any{"sections": []any{map[string]any{
			"section": "mcp_group_id",
			"status":  "failed",
			"error":   "mcp group could not be bound",
			"retry":   "PATCH /api/v1/agents/" + opaqueID + "/mcp-group",
		}}}
	}
	if m.selfHostedTriggerFails {
		rec["triggers"] = []any{map[string]any{
			"name":   "nightly",
			"status": "failed",
			"type":   "schedule",
			"error":  "cron rejected",
			"retry":  "POST /api/v1/agents/" + opaqueID + "/triggers",
		}}
	}
	if m.selfHostedMcpGroupFails || m.selfHostedTriggerFails {
		writeJSON(w, http.StatusMultiStatus, rec)
		return
	}
	writeJSON(w, http.StatusCreated, rec)
}

func firstN(s string, n int) string {
	if len(s) < n {
		return s
	}
	return s[:n]
}

func (m *mockServer) archiveRuntimeAgent(w http.ResponseWriter, agentID string) {
	if rec, ok := m.selfHostedAgs[agentID]; ok {
		rec["is_archived"] = true
		rec["status"] = "archived"
		rec["instances_total"] = 0
	}
	writeJSON(w, http.StatusOK, map[string]any{"agent_id": agentID, "status": "archived"})
}

// deleteAgentMode selects how the mock answers DELETE /api/v1/agents/{id}. The
// real route is gated on the self_hosted_agent_lifecycle flag and 404s while the
// flag is off, which is indistinguishable by status alone from the agent being
// gone. 404 and not a 5xx throughout, because the client retries 5xx away.
type deleteAgentMode string

const (
	// deleteAgentNormal is the default: 204, and the record is removed.
	deleteAgentNormal deleteAgentMode = ""
	// deleteAgentLifecycleOff is the feature gate shut — 404 carrying the control
	// plane's detail string, and the agent survives.
	deleteAgentLifecycleOff deleteAgentMode = "lifecycle_off"
	// deleteAgentOpaque404 is a 404 whose body says nothing useful while the agent
	// survives, so only a follow-up read can tell it from "already gone".
	deleteAgentOpaque404 deleteAgentMode = "opaque_404"
	// deleteAgentAlreadyGone is the genuine race: the record is gone and the delete
	// 404s, which a destroy must keep accepting silently.
	deleteAgentAlreadyGone deleteAgentMode = "already_gone"
	// deleteAgentGateOnMissing is both at once: the control plane checks the gate
	// before it looks the agent up, so an agent that is not there gets the same
	// gate 404 as a live one. The record is dropped before the answer, so the
	// follow-up read 404s and the destroy has nothing left to do.
	deleteAgentGateOnMissing deleteAgentMode = "gate_on_missing"
)

func (m *mockServer) deleteRuntimeAgent(w http.ResponseWriter, agentID string) {
	switch m.deleteAgentAnswer {
	case deleteAgentLifecycleOff:
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": "self-hosted agent lifecycle is not enabled for this account"})
		return
	case deleteAgentOpaque404:
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": "Not Found"})
		return
	case deleteAgentGateOnMissing:
		m.forgetRuntimeAgent(agentID)
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": "self-hosted agent lifecycle is not enabled for this account"})
		return
	}
	m.forgetRuntimeAgent(agentID)
	if m.deleteAgentAnswer == deleteAgentAlreadyGone {
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": "agent not found"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// deleteRuntimeAgentDraft serves DELETE /agents/{id}/draft, the delete that is not
// behind self_hosted_agent_lifecycle. It hard-deletes an agent that has never
// heartbeated and 409s for one that has.
func (m *mockServer) deleteRuntimeAgentDraft(w http.ResponseWriter, agentID string) {
	if m.deletedAgs[agentID] {
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": "agent not found"})
		return
	}
	if m.heartbeatedAgs[agentID] {
		writeJSON(w, http.StatusConflict, map[string]any{"detail": "agent is not a draft"})
		return
	}
	m.forgetRuntimeAgent(agentID)
	w.WriteHeader(http.StatusNoContent)
}

// markAgentHeartbeated stands in for a worker that has come online, which puts the
// agent out of reach of the draft delete. Marked under every key the record
// answers to, since a caller may hold either the slug or the opaque id.
func (m *mockServer) markAgentHeartbeated(agentID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.heartbeatedAgs[agentID] = true
	if rec, ok := m.selfHostedAgs[agentID]; ok {
		m.heartbeatedAgs[toString(rec["id_slug"])] = true
		m.heartbeatedAgs[toString(rec["agent_id"])] = true
	}
}

// forgetRuntimeAgent drops a self-hosted agent under every key it answers to and
// remembers it as deleted, so a later read 404s instead of falling through to the
// synthesized record. Called with m.mu already held by the handler.
func (m *mockServer) forgetRuntimeAgent(agentID string) {
	if rec, ok := m.selfHostedAgs[agentID]; ok {
		delete(m.selfHostedAgs, toString(rec["id_slug"]))
		delete(m.selfHostedAgs, toString(rec["agent_id"]))
		m.deletedAgs[toString(rec["id_slug"])] = true
		m.deletedAgs[toString(rec["agent_id"])] = true
	}
	m.deletedAgs[agentID] = true
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
			if m.hostedPollsDeploying > 0 {
				m.hostedPollsDeploying--
			} else {
				rec["status"] = "online"
			}
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
		// Routing order matches the control plane: an in-flight deploy is abandoned
		// without revoking the worker token, and only a settled deploy reaches the
		// archive gate and the decommission that does revoke.
		if rec["deployStatus"] == "in_progress" {
			delete(m.hostedAgs, key)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if m.deleteNeedsArchive && rec["isArchived"] != true {
			writeJSON(w, http.StatusConflict, map[string]any{"detail": "archive the agent before deleting it"})
			return
		}
		m.revokedTokens[toString(rec["runtimeAgentId"])] = true
		delete(m.hostedAgs, key)
		w.WriteHeader(http.StatusNoContent)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{})
	}
}

// mcpServerCount reports how many gateway servers the control plane still holds,
// for assertions that a create left nothing behind.
func (m *mockServer) mcpServerCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.stores["/api/v1/gateway/admin/servers"])
}

// runtimeAgentCount reports how many distinct agents are registered. Each is
// stored under both its opaque id and its slug.
func (m *mockServer) runtimeAgentCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	seen := map[string]bool{}
	for _, rec := range m.selfHostedAgs {
		seen[toString(rec["agent_id"])] = true
	}
	return len(seen)
}

// tokenRevoked reports whether the delete revoked this agent's worker token.
func (m *mockServer) tokenRevoked(runtimeAgentID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.revokedTokens[runtimeAgentID]
}

// hostedAgentCount reports how many hosted agents the server still holds, for
// destroy assertions.
func (m *mockServer) hostedAgentCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.hostedAgs)
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
