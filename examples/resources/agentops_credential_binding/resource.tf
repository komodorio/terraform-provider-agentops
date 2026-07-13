# Bind a credential to an agent so the agent can use it.
resource "agentops_credential_binding" "openai_to_agent" {
  credential_id = agentops_credential.openai.id
  agent_id      = "agent_01hxyz"
  on_demand     = false
}
