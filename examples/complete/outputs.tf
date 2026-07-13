output "available_integrations" {
  description = "Providers from the integration catalog that are available."
  value       = [for e in data.agentops_integration_catalog.all.entries : e.provider if e.available]
}

output "ci_api_key" {
  description = "The minted API key secret (returned once, at creation)."
  value       = agentops_api_key.ci.key
  sensitive   = true
}

output "deploy_trigger_token" {
  description = "The webhook invocation token for the deploy trigger."
  value       = agentops_trigger.deploy.token
  sensitive   = true
}
