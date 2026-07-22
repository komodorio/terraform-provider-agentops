# Self-contained incident pipeline: Terraform creates the credential and the
# orchestrator + specialist agents, then wires them into the pipeline. No
# pre-existing IDs required.

# The agents' LLM key. credential_ref resolves this credential by NAME, so the
# key must match the provider of the `model` each agent runs (an Anthropic key
# for the claude-sonnet-5 model below).
resource "agentops_credential" "ai_api_key" {
  name  = "incident-agent-ai-api-key"
  value = var.ai_api_key # write-only; never read back
}

# ── Agents the pipeline drives ───────────────────────────────────────────────
resource "agentops_hosted_agent" "orchestrator" {
  customer       = "acme"
  agent_id       = "incident-orchestrator"
  instructions   = "Triage inbound production alerts and delegate to specialists."
  credential_ref = agentops_credential.ai_api_key.name
  model          = "claude-sonnet-5"
}

resource "agentops_hosted_agent" "db_specialist" {
  customer       = "acme"
  agent_id       = "incident-db-specialist"
  instructions   = "Investigate database-related incidents."
  credential_ref = agentops_credential.ai_api_key.name
  model          = "claude-sonnet-5"
}

resource "agentops_hosted_agent" "net_specialist" {
  customer       = "acme"
  agent_id       = "incident-net-specialist"
  instructions   = "Investigate networking-related incidents."
  credential_ref = agentops_credential.ai_api_key.name
  model          = "claude-sonnet-5"
}

# ── Pipeline wiring the agents together ──────────────────────────────────────
resource "agentops_incident_pipeline" "prod" {
  name   = "production-incidents"
  status = "active"

  alert_source = {
    provider     = "generic"
    monitor_mode = "create_catchall"
  }

  routing_rule = {
    environment = "production"
    severity    = "critical"
  }

  orchestrator_binding = {
    agent_id = agentops_hosted_agent.orchestrator.id
  }

  specialist_bindings = [
    {
      agent_id = agentops_hosted_agent.db_specialist.id
      role     = "database"
      enabled  = true
    },
    {
      agent_id = agentops_hosted_agent.net_specialist.id
      role     = "networking"
      enabled  = true
    },
  ]
}
