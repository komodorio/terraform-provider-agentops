# RBAC scenario: build an authorization model from the ground up and hand it to
# both a machine identity (service account + API key) and a human (member).
#
#   policy (capabilities) -> role (bundle of policies) -> grant (role on a
#   resource, to a subject)
#
# Two subjects receive the same operator role: a CI service account scoped to a
# single agent, and a human member scoped to all agents.

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

# ── Capabilities: what actions are allowed ───────────────────────────────────
resource "agentops_policy" "operate" {
  name        = "operate-agents"
  description = "Invoke and inspect agents"

  grants = jsonencode([
    { capability = "agent.invoke", resource_type = "agent" },
    { capability = "agent.read", resource_type = "agent" },
  ])
}

# ── Role: a reusable bundle of policies ──────────────────────────────────────
resource "agentops_role" "operator" {
  name        = "operator"
  description = "Can operate agents"
  policy_ids  = [agentops_policy.operate.id]
}

# ── Machine identity: service account + API key ──────────────────────────────
resource "agentops_service_account" "ci" {
  display_name = "ci-bot"
}

resource "agentops_api_key" "ci" {
  name               = "ci-pipeline"
  service_account_id = agentops_service_account.ci.id
  scopes             = ["triggers:write"]
}

# The service account may operate one specific agent.
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

# ── Human identity: member ───────────────────────────────────────────────────
resource "agentops_member" "on_call" {
  email     = var.member_email
  full_name = "On-Call Engineer"
}

# The on-call engineer may operate every agent (resource_id = "*").
resource "agentops_grant" "on_call_operator" {
  grant_kind    = "role"
  role_id       = agentops_role.operator.id
  resource_type = "agent"
  resource_id   = "*"

  subject = jsonencode({
    id   = agentops_member.on_call.id
    kind = "principal"
  })
}
