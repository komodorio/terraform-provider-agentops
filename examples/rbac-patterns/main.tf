# RBAC scenario — grants scoped by name glob.
#
# `resource_id` is exact-or-"*" only; it is never a glob. To scope a grant to a
# set of resources by name pattern, put the glob in the selector under the
# reserved `name_glob` key and leave resource_id as "*":
#
#   resource_id = "*", selector = { name_glob = "agent_prod-*" }
#
# `*` is the only wildcard (every other character is literal), so "agent_prod-*"
# matches agent_prod-checkout but not agent_dev-sandbox. The invariant is
# name_glob ⇔ resource_id == "*": a name_glob is only valid with a "*" id.
#
# (name_glob is identity matching; ABAC *label* selectors use other keys — see
# the rbac-labels example.)

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

# ── Policy: capabilities scoped to production agents by name glob ─────────────
resource "agentops_policy" "prod_agents" {
  name        = "prod-agent-operator"
  description = "Operate only production agents, matched by name glob"

  # Each grant keeps resource_id = "*" and carries the glob in selector.name_glob.
  grants = jsonencode([
    { capability = "agent.read", resource_type = "agent", resource_id = "*", selector = { name_glob = "agent_prod-*" } },
    { capability = "agent.invoke", resource_type = "agent", resource_id = "*", selector = { name_glob = "agent_prod-*" } },
  ])
}

resource "agentops_role" "prod_operator" {
  name        = "prod-operator"
  description = "Operates production agents"
  policy_ids  = [agentops_policy.prod_agents.id]
}

# ── Subject the role is bound to ─────────────────────────────────────────────
resource "agentops_service_account" "ci" {
  display_name = "prod-ci"
}

# The binding grant is glob-scoped the same way: resource_id "*" plus a
# name_glob selector, so the role only takes effect on agent_prod-* resources.
resource "agentops_grant" "ci_prod_operator" {
  grant_kind    = "role"
  role_id       = agentops_role.prod_operator.id
  resource_type = "agent"
  resource_id   = "*"

  selector = jsonencode({ name_glob = "agent_prod-*" })

  subject = jsonencode({
    id   = agentops_service_account.ci.id
    kind = "principal"
  })
}

# A standalone capability grant can be name-glob-scoped too, without a policy.
resource "agentops_grant" "read_prod_agents" {
  grant_kind    = "capability"
  capability    = "agent.read"
  resource_type = "agent"
  resource_id   = "*"

  selector = jsonencode({ name_glob = "agent_prod-*" })

  subject = jsonencode({
    id   = agentops_service_account.ci.id
    kind = "principal"
  })
}
