# Grant an agent access to a knowledge base.
resource "agentops_knowledge_base_agent" "runbooks_to_agent" {
  kb_id    = agentops_knowledge_base.runbooks.id
  agent_id = "agent_01hxyz"
}
