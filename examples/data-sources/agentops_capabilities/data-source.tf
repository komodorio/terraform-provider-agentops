# All authorization capabilities known to the account.
data "agentops_capabilities" "all" {}

output "capability_keys" {
  value = [for c in data.agentops_capabilities.all.capabilities : c.key]
}
