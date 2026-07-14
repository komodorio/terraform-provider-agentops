output "prod_server_id" {
  description = "ID of the prod-labelled server the selector grant matches."
  value       = agentops_mcp_gateway_server.prod_docs.id
}

output "grant_id" {
  description = "ID of the label-scoped grant."
  value       = agentops_grant.prod_servers.id
}
