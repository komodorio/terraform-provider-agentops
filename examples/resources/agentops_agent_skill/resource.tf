# Attach an authored skill to an agent. Omit pin_version_id to float on the
# latest published version.
resource "agentops_agent_skill" "runbook_to_agent" {
  agent_id = "agent_01hxyz"
  skill_id = agentops_skill.deploy_runbook.id
}
