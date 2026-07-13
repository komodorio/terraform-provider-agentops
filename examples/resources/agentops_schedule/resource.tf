resource "agentops_schedule" "nightly" {
  agent_id  = "agent_01hxyz"
  cron_expr = "0 2 * * *"
  timezone  = "UTC"
  input     = jsonencode({ mode = "full" })
}
