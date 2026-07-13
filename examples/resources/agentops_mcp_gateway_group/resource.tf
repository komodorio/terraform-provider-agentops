resource "agentops_mcp_gateway_group" "core" {
  name              = "core-tools"
  member_server_ids = [agentops_mcp_gateway_server.docs.id]
}
