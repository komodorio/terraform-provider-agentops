# End-to-end example: discover available integrations, mint an API key for a
# service account, and register a webhook trigger for an agent.
#
# As more resources land (service accounts, roles, workflows), this example will
# grow into the full chain: create service account -> grant role -> mint api key
# -> connect integration -> wire a workflow.

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

# Look up the integrations available to this account.
data "agentops_integration_catalog" "all" {}

# Mint an API key for a CI pipeline, bound to a service account.
resource "agentops_api_key" "ci" {
  name               = "ci-pipeline"
  service_account_id = var.service_account_id
  scopes             = ["triggers:write"]
}

# Register a webhook trigger that invokes an agent.
resource "agentops_trigger" "deploy" {
  name        = "deploy-webhook"
  description = "Fires the deploy agent on inbound webhooks"
  target_id   = var.agent_id
  target_type = "agent"
  is_enabled  = true
}
