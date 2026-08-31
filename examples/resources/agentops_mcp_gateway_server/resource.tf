resource "agentops_mcp_gateway_server" "docs" {
  name = "docs-mcp"
  url  = "https://mcp.example.com"

  # Static token auth via a secret reference resolved at connect time.
  static_headers = {
    Authorization = "Bearer $${env:DOCS_MCP_TOKEN}"
  }

  allow = ["search_*"]
}

# A ${credential:NAME} reference in static_headers only resolves on a server with a
# credential bound to it. Without the binding below, the gateway sends the literal
# text "${credential:grafana-token}" upstream as the header value.
resource "agentops_credential" "grafana_token" {
  name  = "grafana-token"
  value = var.grafana_service_account_token
}

resource "agentops_mcp_gateway_server" "grafana" {
  name = "grafana-mcp"
  url  = "https://grafana.example.com/mcp"

  credential_source_id = agentops_credential.grafana_token.id

  static_headers = {
    "X-Grafana-Service-Account-Token" = "$${credential:grafana-token}"
  }
}
