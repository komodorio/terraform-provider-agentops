# RBAC scenario — resource patterns.
#
# Instead of granting access to one resource at a time, grants can target every
# resource of a type with the "*" wildcard, and can be issued directly as a
# single capability (grant_kind = "capability") rather than through a role.
#
# This example shows both grant kinds against a wildcard resource pattern:
#   - a direct capability grant (read all agents)
#   - a role grant (operate all agents)

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

# The subject both grants are bound to.
resource "agentops_service_account" "fleet_ops" {
  display_name = "fleet-ops"
}

# ── Direct capability grant: no role, one capability, all agents ─────────────
# grant_kind = "capability" binds a single capability straight to the subject.
# resource_id = "*" is the wildcard pattern: every agent, current and future.
resource "agentops_grant" "read_all_agents" {
  grant_kind    = "capability"
  capability    = "agent.read"
  resource_type = "agent"
  resource_id   = "*"

  subject = jsonencode({
    id   = agentops_service_account.fleet_ops.id
    kind = "principal"
  })
}

# ── Role grant over the same wildcard pattern ────────────────────────────────
resource "agentops_policy" "operate" {
  name        = "operate-agents"
  description = "Invoke and read agents"

  grants = jsonencode([
    { capability = "agent.invoke", resource_type = "agent" },
    { capability = "agent.read", resource_type = "agent" },
  ])
}

resource "agentops_role" "operator" {
  name        = "operator"
  description = "Can operate agents"
  policy_ids  = [agentops_policy.operate.id]
}

# grant_kind = "role" applies the whole role over the wildcard pattern. To scope
# a grant to a single resource instead, set resource_id to that resource's id.
resource "agentops_grant" "operate_all_agents" {
  grant_kind    = "role"
  role_id       = agentops_role.operator.id
  resource_type = "agent"
  resource_id   = "*"

  subject = jsonencode({
    id   = agentops_service_account.fleet_ops.id
    kind = "principal"
  })
}
