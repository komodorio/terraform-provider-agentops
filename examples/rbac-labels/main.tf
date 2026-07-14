# RBAC scenario — attribute-based access with labels.
#
# Resources carry arbitrary key/value labels; a grant's `selector` matches those
# labels instead of a hard-coded resource id. The grant then follows the labels:
# any resource that gains the matching labels is covered automatically, and any
# that loses them drops out — no Terraform change needed.
#
# Here two MCP gateway servers are labelled by environment, and a role is granted
# over "prod"-labelled servers via a selector.

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

# ── Labelled resources ───────────────────────────────────────────────────────
resource "agentops_mcp_gateway_server" "prod_docs" {
  name = "prod-docs-mcp"
  url  = "https://docs-mcp.example.com"

  labels = {
    env  = "prod"
    team = "sre"
  }
}

resource "agentops_mcp_gateway_server" "staging_docs" {
  name = "staging-docs-mcp"
  url  = "https://staging-docs-mcp.example.com"

  labels = {
    env  = "staging"
    team = "sre"
  }
}

# ── Authorization model ──────────────────────────────────────────────────────
resource "agentops_policy" "operate_servers" {
  name        = "operate-mcp-servers"
  description = "Read and invoke MCP gateway servers"

  grants = jsonencode([
    { capability = "mcp_server.read", resource_type = "mcp_server" },
    { capability = "mcp_server.invoke", resource_type = "mcp_server" },
  ])
}

resource "agentops_role" "server_operator" {
  name        = "mcp-server-operator"
  description = "Operates MCP gateway servers"
  policy_ids  = [agentops_policy.operate_servers.id]
}

resource "agentops_service_account" "prod_ops" {
  display_name = "prod-ops"
}

# ── Label-scoped grant ───────────────────────────────────────────────────────
# resource_id = "*" spans every server; the selector narrows that to the ones
# whose labels match (env = prod, team = sre). The server that carries those
# labels — prod_docs — is covered; staging_docs is not.
resource "agentops_grant" "prod_servers" {
  grant_kind    = "role"
  role_id       = agentops_role.server_operator.id
  resource_type = "mcp_server"
  resource_id   = "*"

  # Free-form attribute selector: matches resources by their labels.
  selector = jsonencode({
    env  = "prod"
    team = "sre"
  })

  subject = jsonencode({
    id   = agentops_service_account.prod_ops.id
    kind = "principal"
  })
}
