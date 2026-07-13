resource "agentops_mcp_gateway_server" "docs" {
  name = "docs-mcp"
  url  = "https://mcp.example.com"

  # Static token auth via a secret reference resolved at connect time.
  static_headers = {
    Authorization = "Bearer $${env:DOCS_MCP_TOKEN}"
  }

  allow = ["search_*"]
}
