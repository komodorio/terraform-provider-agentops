# A webhook trigger that invokes an agent when called.
resource "agentops_trigger" "deploy" {
  name        = "deploy-webhook"
  description = "Fires the deploy agent on inbound webhooks"
  target_id   = "agent_01hxyz"
  target_type = "agent"
  is_enabled  = true
}

# The invocation token is returned only at create time; keep it secret.
output "deploy_trigger_token" {
  value     = agentops_trigger.deploy.token
  sensitive = true
}
