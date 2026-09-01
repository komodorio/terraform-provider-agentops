output "skill_id" {
  description = "ID of the authored skill."
  value       = agentops_skill.deploy_runbook.id
}

output "content_version" {
  description = "Latest published content version number. Increments each time the skill's content changes."
  value       = agentops_skill.deploy_runbook.content_version
}

output "binding_id" {
  description = "ID of the agent<->skill binding."
  value       = agentops_agent_skill.runbook_to_agent.id
}
