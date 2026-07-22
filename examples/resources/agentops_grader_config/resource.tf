resource "agentops_grader_config" "prod_quality" {
  target_agent_id = "agent_01hxyz"
  grader_agent_id = "agent_grader_01"
  sample_rate     = 25
  guidelines      = "Penalize hallucinated remediation steps; reward citing the runbook."
}
