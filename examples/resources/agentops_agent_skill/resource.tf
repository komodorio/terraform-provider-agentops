# Attach an authored skill to an agent. Omit pin_version to float on the latest
# published version, or set it to a published version number to pin the agent.
resource "agentops_agent_skill" "runbook_to_agent" {
  agent_id    = "agent_01hxyz"
  skill_id    = agentops_skill.deploy_runbook.id
  pin_version = 3 # optional; omit to always follow the latest version
}
