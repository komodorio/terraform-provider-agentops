resource "agentops_mcp_gateway_policy" "default_deny" {
  enabled = true

  document = jsonencode({
    description    = "Default deny with an allowlist"
    tool_allowlist = ["search_docs"]
  })
}
