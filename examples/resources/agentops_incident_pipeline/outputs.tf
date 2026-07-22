# Handy outputs for driving the pipeline once it's applied. Fetch the token with
# `terraform output -raw incident_webhook_token` (it's sensitive), then POST an
# alert to the URL with the token in the `X-Webhook-Token` header.

output "incident_webhook_url" {
  description = "Endpoint to POST alerts to. Feeds the active incident pipeline."
  value       = agentops_incident_pipeline.prod.webhook_url
}

output "incident_webhook_token" {
  description = "Token for the X-Webhook-Token header when posting to the endpoint. Returned only at create/rotation."
  value       = agentops_trigger.incidents_endpoint.token
  sensitive   = true
}

output "incident_pipeline_id" {
  description = "ID of the incident pipeline (use for imports or API calls)."
  value       = agentops_incident_pipeline.prod.id
}

output "incident_trigger_id" {
  description = "ID of the webhook trigger backing the pipeline."
  value       = agentops_trigger.incidents_endpoint.id
}

output "orchestrator_runtime_agent_id" {
  description = "Runtime agent ID the orchestrator dispatches runs as."
  value       = agentops_worker_catalog_deployment.orchestrator.runtime_agent_id
}
