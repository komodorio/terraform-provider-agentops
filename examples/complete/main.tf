# End-to-end example wiring the AgentOps config plane together:
# policy -> role -> service account -> grant -> API key -> credential (+ binding)
# -> knowledge base (+ agent grant) -> integration -> workflow -> trigger ->
# schedule, plus a couple of read-only lookups.

terraform {
  required_providers {
    agentops = {
      source = "komodorio/agentops"
    }
  }
}

provider "agentops" {
  endpoint = var.endpoint # optional; defaults to https://agentops.komodor.com
  api_key  = var.api_key
}

# ── Read-only lookups ────────────────────────────────────────────────────────
data "agentops_integration_catalog" "all" {}
data "agentops_capabilities" "all" {}

# ── Authorization: policy -> role -> service account -> grant ────────────────
resource "agentops_policy" "invoke" {
  name        = "invoke-agents"
  description = "Allows invoking agents"

  grants = jsonencode([
    { capability = "agent.invoke", resource_type = "agent" },
  ])
}

resource "agentops_role" "operator" {
  name        = "operator"
  description = "Can operate agents"
  policy_ids  = [agentops_policy.invoke.id]
}

resource "agentops_service_account" "ci" {
  display_name = "ci-bot"
}

resource "agentops_grant" "ci_operator" {
  grant_kind    = "role"
  role_id       = agentops_role.operator.id
  resource_type = "agent"
  resource_id   = var.agent_id

  subject = jsonencode({
    id   = agentops_service_account.ci.id
    kind = "principal"
  })
}

# ── Credentials for the service account / agents ─────────────────────────────
resource "agentops_api_key" "ci" {
  name               = "ci-pipeline"
  service_account_id = agentops_service_account.ci.id
  scopes             = ["triggers:write"]
}

resource "agentops_credential" "openai" {
  name  = "openai-api-key"
  value = var.openai_api_key
}

resource "agentops_credential_binding" "openai_to_agent" {
  credential_id = agentops_credential.openai.id
  agent_id      = var.agent_id
}

# ── Knowledge base + agent access ────────────────────────────────────────────
resource "agentops_knowledge_base" "runbooks" {
  name        = "runbooks"
  description = "Operational runbooks"
}

resource "agentops_knowledge_base_agent" "runbooks_access" {
  kb_id    = agentops_knowledge_base.runbooks.id
  agent_id = var.agent_id
}

# ── Automation: workflow + trigger + schedule ────────────────────────────────
resource "agentops_workflow" "triage" {
  name       = "incident-triage"
  is_enabled = true
}

resource "agentops_trigger" "deploy" {
  name        = "deploy-webhook"
  target_id   = var.agent_id
  target_type = "agent"
}

resource "agentops_schedule" "nightly" {
  agent_id  = var.agent_id
  cron_expr = "0 2 * * *"
}
