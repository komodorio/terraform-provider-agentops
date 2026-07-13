# An API key bound to a service account. API keys cannot be updated in place;
# changing any argument replaces the key. The secret is returned only once.
resource "agentops_api_key" "ci" {
  name               = "ci-pipeline"
  service_account_id = "sa_01hxyz"
  scopes             = ["triggers:write"]
}

output "ci_api_key" {
  value     = agentops_api_key.ci.key
  sensitive = true
}
