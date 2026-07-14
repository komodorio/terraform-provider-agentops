output "group_id" {
  description = "ID of the core-tools gateway group agents attach to."
  value       = agentops_mcp_gateway_group.core.id
}

output "server_ids" {
  description = "IDs of the upstream MCP servers registered with the gateway."
  value = {
    docs   = agentops_mcp_gateway_server.docs.id
    issues = agentops_mcp_gateway_server.issues.id
  }
}
