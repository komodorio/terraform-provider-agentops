# Self-contained incident pipeline: Terraform deploys the orchestrator from the
# worker catalog, creates the specialist agents (and their LLM credential), then
# wires them into the pipeline. No pre-existing IDs required. Each agent's
# `customer` (tenant) is derived from your account by the server.

# The specialists' LLM key. credential_ref resolves this credential by NAME, so
# the key must match the provider of the `model` each agent runs (an Anthropic
# key for the claude-sonnet-5 model below).
resource "agentops_credential" "specialist_llm_key" {
  name  = "specialist-llm-key"
  value = var.specialist_llm_key # write-only; never read back
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
  credential_ref = agentops_credential.specialist_llm_key.name
  model          = "claude-sonnet-5"
}

resource "agentops_hosted_agent" "net_specialist" {
  agent_id       = "incident-net-specialist"
  instructions   = "Investigate networking-related incidents."
  credential_ref = agentops_credential.specialist_llm_key.name
  model          = "claude-sonnet-5"
}

# ── Webhook endpoint that feeds alerts into the pipeline ─────────────────────
# A standalone webhook trigger (no target); the pipeline consumes it via
# trigger_id. An endpoint must be linked before the pipeline can be activated.
resource "agentops_trigger" "incidents_endpoint" {
  name        = "incident-alerts"
  description = "Inbound alert webhook for the staging incident pipeline"
}

# ── Pipeline wiring the agents together ──────────────────────────────────────
resource "agentops_incident_pipeline" "incidents" {
  name       = "staging-incidents"
  status     = "active"
  trigger_id = agentops_trigger.incidents_endpoint.id

  alert_source = {
    provider     = "generic"
    monitor_mode = "create_catchall"
  }

  routing_rule = {
    environment = "staging"
    severity    = "critical"
  }

  # Bindings reference the agent's runtime_agent_id — the opaque id the run
  # dispatcher resolves against — NOT the resource `id` (a hosted-agent record
  # PK, which the dispatcher can't resolve, so alerts would create an incident
  # but never dispatch the orchestrator run).
  orchestrator_binding = {
    agent_id = agentops_worker_catalog_deployment.orchestrator.runtime_agent_id
  }

  specialist_bindings = [
    {
      agent_id = agentops_hosted_agent.db_specialist.runtime_agent_id
      role     = "database"
      enabled  = true
    },
    {
      agent_id = agentops_hosted_agent.net_specialist.runtime_agent_id
      role     = "networking"
      enabled  = true
    },
  ]
}
