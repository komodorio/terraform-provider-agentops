# MCP gateway scenario: expose upstream MCP servers to agents through the
# gateway, bundle them into a group, and gate tool access with a policy.
#
#   servers (upstream MCP endpoints) -> group (a bundle agents can attach to)
#   + policy (allow/deny which tools may be called)

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

# ── Upstream MCP servers ─────────────────────────────────────────────────────
resource "agentops_mcp_gateway_server" "docs" {
  name = "docs-mcp"
  url  = "https://docs-mcp.example.com"

  # Static token auth via a secret reference resolved at connect time.
  static_headers = {
    Authorization = "Bearer $${env:DOCS_MCP_TOKEN}"
  }

  # Only expose the read-oriented search tools from this server.
  allow = ["search_*"]
}

resource "agentops_mcp_gateway_server" "issues" {
  name = "issues-mcp"
  url  = "https://issues-mcp.example.com"

  static_headers = {
    Authorization = "Bearer $${env:ISSUES_MCP_TOKEN}"
  }
}

# ── Group: a bundle of servers agents can attach to ──────────────────────────
resource "agentops_mcp_gateway_group" "core" {
  name = "core-tools"
  member_server_ids = [
    agentops_mcp_gateway_server.docs.id,
    agentops_mcp_gateway_server.issues.id,
  ]
}

# ── Policy: default-deny allowlist across the gateway ────────────────────────
resource "agentops_mcp_gateway_policy" "default_deny" {
  enabled = true

  document = jsonencode({
    description = "Default deny; only allow doc search and issue reads"
    tool_allowlist = [
      "search_docs",
      "get_issue",
    ]
  })
}
