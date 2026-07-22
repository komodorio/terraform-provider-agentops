resource "agentops_hosted_agent" "triage" {
  # customer is optional: the server derives it from your account when omitted.
  agent_id       = "incident-triage"
  instructions   = "Triage incoming production alerts and page the on-call when severity is high."
  credential_ref = "cred_01hxyz"
  model          = "claude-sonnet-5"

  skills = [
    {
      id      = "runbook-search"
      content = file("${path.module}/skills/runbook-search.md")
    },
  ]

  image = {
    repository = "komodorio/hosted-agent"
    tag        = "v1.4.2"
  }
}
