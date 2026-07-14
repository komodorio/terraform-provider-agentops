output "deploy_trigger_token" {
  description = "The webhook invocation token for the deploy trigger (returned once)."
  value       = agentops_trigger.deploy.token
  sensitive   = true
}

output "schedule_id" {
  description = "ID of the nightly schedule."
  value       = agentops_schedule.nightly.id
}
