# Requires an active Slack connector on the account. Connect Slack under
# Settings -> Integrations first; without one the create is refused.

# Anyone mentioning the AgentOps bot in #prod-oncall opens an incident.
resource "agentops_incident_pipeline_slack_trigger" "oncall" {
  pipeline_id = agentops_incident_pipeline.prod.id
  channel_id  = "C0123ABCDEF"
  rule_type   = "mention"
}

# A keyword rule reads the word it watches for from `match`.
resource "agentops_incident_pipeline_slack_trigger" "oom" {
  pipeline_id = agentops_incident_pipeline.prod.id
  channel_id  = "C0123ABCDEF"
  rule_type   = "keyword"

  match = jsonencode({
    keyword = "oomkilled"
  })
}
