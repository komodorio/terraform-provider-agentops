output "policy_id" {
  description = "ID of the glob-scoped production policy."
  value       = agentops_policy.prod_agents.id
}

output "grant_ids" {
  description = "IDs of the role binding and standalone capability grants."
  value = {
    role_binding = agentops_grant.ci_prod_operator.id
    read_prod    = agentops_grant.read_prod_agents.id
  }
}
