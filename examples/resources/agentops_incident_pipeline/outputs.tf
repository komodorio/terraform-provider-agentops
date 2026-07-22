# Handy outputs for driving the pipeline once it's applied. Fetch the token with
# `terraform output -raw incident_webhook_token` (it's sensitive), then POST an
# alert to the URL with the token in the `X-Webhook-Token` header. The
# `how_to_test` output prints a ready-to-run command after every apply.

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

# The runtime_agent_id is the id the run dispatcher resolves against (what the
# pipeline binds to). Exposed for all three agents so you can verify runs/logs.
output "orchestrator_runtime_agent_id" {
  description = "Runtime agent ID the orchestrator dispatches runs as."
  value       = agentops_worker_catalog_deployment.orchestrator.runtime_agent_id
}

output "db_specialist_runtime_agent_id" {
  description = "Runtime agent ID of the database specialist."
  value       = agentops_hosted_agent.db_specialist.runtime_agent_id
}

output "net_specialist_runtime_agent_id" {
  description = "Runtime agent ID of the networking specialist."
  value       = agentops_hosted_agent.net_specialist.runtime_agent_id
}

# Printed after every successful apply: a copy-paste test that fires an alert
# matching the routing rule (environment=production, severity=critical) so the
# pipeline creates an incident and dispatches the orchestrator.
output "how_to_test" {
  description = "Steps to fire a test alert through the pipeline."
  value       = <<-EOT

    ── Test the incident pipeline ──────────────────────────────────────────────
    1. Grab the webhook token (sensitive, so fetch it explicitly):

         TOKEN=$(terraform output -raw incident_webhook_token)

    2. Fire a test alert. The labels below match the pipeline's routing rule
       (environment=production, severity=critical); use a fresh "fingerprint"
       each time — re-firing the same one updates the incident but does NOT
       dispatch a new orchestrator run:

         curl -sS -X POST "${agentops_incident_pipeline.prod.webhook_url}" \
           -H "X-Webhook-Token: $TOKEN" \
           -H "Content-Type: application/json" \
           -d '{
             "status": "firing",
             "labels": { "alertname": "HighErrorRate", "env": "production", "severity": "critical" },
             "annotations": { "description": "Error rate exceeded threshold on checkout-service" },
             "fingerprint": "test-'"$(date +%s)"'"
           }'

       Expect HTTP 202. If you get 401, the token is wrong; if 202 but no
       incident, the labels didn't match the routing rule.

    3. Open /incidents in the AgentOps UI and check that the incident appears
       and moves through its status as the orchestrator runs.
    ────────────────────────────────────────────────────────────────────────────
  EOT
}
