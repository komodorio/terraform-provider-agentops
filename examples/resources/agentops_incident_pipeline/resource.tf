# Self-contained incident pipeline: Terraform deploys the orchestrator from the
# worker catalog, creates the specialist agents (and their LLM credential), then
# wires them into the pipeline. No pre-existing IDs required. Each agent's
# `customer` (tenant) is derived from your account by the server.

# The specialists' LLM key. credential_ref resolves this credential by NAME, so
# the key must match the provider of the `model` each agent runs (an Anthropic
# key for the claude-sonnet-5 model below).
resource "agentops_credential" "ai_api_key" {
  name  = "incident-agent-ai-api-key"
  value = var.ai_api_key # write-only; never read back
}

# ── Orchestrator: deployed from the worker catalog ───────────────────────────
# The "orchestrator" catalog entry pins the image and uses the managed AgentOps
# LLM gateway, so the customer and credentials are derived server-side — you only
# name the instance.
resource "agentops_worker_catalog_deployment" "orchestrator" {
  catalog_id   = "orchestrator"
  agent_id     = "incident-orchestrator"
  display_name = "Incident Orchestrator"
}

# ── Specialists the orchestrator delegates to ────────────────────────────────
resource "agentops_hosted_agent" "db_specialist" {
  agent_id       = "incident-db-specialist"
  instructions   = "Investigate database-related incidents."
  credential_ref = agentops_credential.ai_api_key.name
  model          = "claude-sonnet-5"
}

resource "agentops_hosted_agent" "net_specialist" {
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
    agent_id = agentops_worker_catalog_deployment.orchestrator.id
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
