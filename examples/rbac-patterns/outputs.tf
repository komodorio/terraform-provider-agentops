output "subject_id" {
  description = "Principal id the wildcard grants are bound to."
  value       = agentops_service_account.fleet_ops.id
}

output "grant_ids" {
  description = "IDs of the capability and role grants."
  value = {
    read_all    = agentops_grant.read_all_agents.id
    operate_all = agentops_grant.operate_all_agents.id
  }
}
