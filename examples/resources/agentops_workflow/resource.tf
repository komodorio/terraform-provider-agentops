resource "agentops_workflow" "triage" {
  name        = "incident-triage"
  description = "Triage inbound incidents"
  is_enabled  = true

  labels = {
    owner = "sre"
  }

  # steps and trigger are free-form JSON payloads.
  trigger = jsonencode({ type = "manual" })
  steps   = jsonencode([{ id = "step-1", agent_id = "agent_01hxyz" }])
}
