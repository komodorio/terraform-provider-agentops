resource "agentops_channel_route" "deploys" {
  channel_id  = agentops_channel.alerts.id
  rule_type   = "keyword"
  target_type = "agent"
  target_id   = "agent_01hxyz"
  priority    = 10

  match = jsonencode({
    keyword = "deploy"
  })
}
