output "ci_api_key" {
  description = "The minted CI API key secret (returned once, at creation)."
  value       = agentops_api_key.ci.key
  sensitive   = true
}

output "operator_role_id" {
  description = "ID of the operator role shared by both grants."
  value       = agentops_role.operator.id
}
