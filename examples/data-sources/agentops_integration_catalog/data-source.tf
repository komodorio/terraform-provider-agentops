# List the integration providers available to this account.
data "agentops_integration_catalog" "all" {}

output "available_integrations" {
  value = [for e in data.agentops_integration_catalog.all.entries : e.provider if e.available]
}
