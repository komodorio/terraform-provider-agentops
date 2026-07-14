# Automation scenario: give an agent a credential, then drive it three ways —
# on demand (workflow), on inbound webhooks (trigger), and on a schedule.
#
#   credential (+ binding) -> agent
#   workflow  (manual/orchestrated run)
#   trigger   (webhook -> agent)
#   schedule  (cron -> agent)

terraform {
  required_providers {
    agentops = {
      source = "komodorio/agentops"
    }
  }
}

provider "agentops" {
  endpoint = var.endpoint # optional; defaults to https://agentops.komodor.com
  api_key  = var.api_key
}

# ── Credential the agent uses at run time ────────────────────────────────────
resource "agentops_credential" "openai" {
  name        = "openai-api-key"
  value       = var.openai_api_key # write-only; never read back
  description = "OpenAI API key for the automation agent"
  labels = {
    team = "platform"
  }
}

resource "agentops_credential_binding" "openai_to_agent" {
  credential_id = agentops_credential.openai.id
  agent_id      = var.agent_id
}

# ── On demand: a multi-step workflow ─────────────────────────────────────────
resource "agentops_workflow" "triage" {
  name        = "incident-triage"
  description = "Triage inbound incidents"
  is_enabled  = true

  labels = {
    owner = "sre"
  }

  trigger = jsonencode({ type = "manual" })
  steps   = jsonencode([{ id = "step-1", agent_id = var.agent_id }])
}

# ── On webhook: a trigger that invokes the agent ─────────────────────────────
resource "agentops_trigger" "deploy" {
  name        = "deploy-webhook"
  description = "Fires the agent on inbound webhooks"
  target_id   = var.agent_id
  target_type = "agent"
  is_enabled  = true
}

# ── On schedule: a nightly cron run ──────────────────────────────────────────
resource "agentops_schedule" "nightly" {
  agent_id  = var.agent_id
  cron_expr = "0 2 * * *"
  timezone  = "UTC"
  input     = jsonencode({ mode = "full" })
}
