# GPU incident-response smoke test — agents, outpost, MCP binding.
#
# Prerequisites:
#   1. Build the provider locally:
#        cd <repo-root> && go install ./...
#   2. Export credentials:
#        export AGENTOPS_ENDPOINT=https://staging.agentops.komodor.com
#        export AGENTOPS_API_KEY=<your staging API key>
#   3. Ensure a credential named ANTHROPIC_API_KEY exists on the account.
#   4. Run:
#        terraform plan
#        terraform apply -auto-approve
#   5. Tear down:
#        terraform destroy -auto-approve

terraform {
  required_providers {
    agentops = {
      source = "komodorio/agentops"
    }
  }
}

provider "agentops" {}

# ── Agents ───────────────────────────────────────────────────────────────────

resource "agentops_agent" "orchestrator" {
  agent_id       = "tf-smoke-orchestrator"
  instructions   = "Triages incidents and delegates to investigators."
  display_name   = "TF Smoke Orchestrator"
  credential_ref = "ANTHROPIC_API_KEY"
  model          = "claude-sonnet-4-20250514"
}

resource "agentops_agent" "investigator" {
  agent_id       = "tf-smoke-investigator"
  instructions   = "Investigates GPU cluster health using Grafana."
  display_name   = "TF Smoke Investigator"
  credential_ref = "ANTHROPIC_API_KEY"
  model          = "claude-sonnet-4-20250514"
}

# ── Outpost ──────────────────────────────────────────────────────────────────

resource "agentops_outpost" "gpu" {
  name        = "tf-smoke-outpost"
  description = "Created by Terraform smoke test — safe to delete"

  allowlist = [
    {
      scheme = "http"
      host   = "mcp-grafana.agentops-outpost.svc.cluster.local"
      port   = 8080
    },
  ]

  labels = {
    env     = "test"
    created = "terraform-smoke"
  }
}

data "agentops_outpost_install" "gpu" {
  outpost_id = agentops_outpost.gpu.id
}

# ── MCP gateway: server → outpost → group → agent binding ───────────────────

resource "agentops_mcp_gateway_server" "grafana" {
  name       = "tf-smoke-grafana"
  url        = "http://mcp-grafana.agentops-outpost.svc.cluster.local:8080/mcp"
  outpost_id = agentops_outpost.gpu.id
}

resource "agentops_mcp_gateway_group" "grafana" {
  name              = "tf-smoke-grafana"
  member_server_ids = [agentops_mcp_gateway_server.grafana.id]
}

resource "agentops_agent" "investigator_with_mcp" {
  agent_id       = "tf-smoke-investigator-mcp"
  instructions   = "Investigates GPU cluster health using Grafana via MCP."
  display_name   = "TF Smoke Investigator (MCP)"
  credential_ref = "ANTHROPIC_API_KEY"
  model          = "claude-sonnet-4-20250514"
  mcp_group_id   = agentops_mcp_gateway_group.grafana.id
}

# ── Outputs ──────────────────────────────────────────────────────────────────

output "orchestrator_agent_id" {
  value = agentops_agent.orchestrator.agent_id
}

output "orchestrator_install_command" {
  value = agentops_agent.orchestrator.install_command
}

output "orchestrator_install_values" {
  value     = agentops_agent.orchestrator.install_values
  sensitive = true
}

output "orchestrator_status" {
  value = agentops_agent.orchestrator.status
}

output "investigator_install_command" {
  value = agentops_agent.investigator.install_command
}

output "outpost_id" {
  value = agentops_outpost.gpu.id
}

output "outpost_credential" {
  value     = agentops_outpost.gpu.credential
  sensitive = true
}

output "outpost_install_command" {
  value = data.agentops_outpost_install.gpu.command
}

output "mcp_server_id" {
  value = agentops_mcp_gateway_server.grafana.id
}

output "mcp_group_id" {
  value = agentops_mcp_gateway_group.grafana.id
}

output "investigator_mcp_binding" {
  value = agentops_agent.investigator_with_mcp.mcp_group_id
}
