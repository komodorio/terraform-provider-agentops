# Deploy a curated worker from the catalog. The server pins the image and derives
# the customer and credentials, so you only name the instance. Browse available
# entries (and their IDs / required credentials) via the agentops_worker_catalog
# data source.
data "agentops_worker_catalog" "all" {}

resource "agentops_worker_catalog_deployment" "k8s_troubleshooter" {
  catalog_id   = "k8s-troubleshooter"
  agent_id     = "prod-k8s-troubleshooter"
  display_name = "Prod K8s Troubleshooter"

  # LLM credential (by name) from the entry's allowed set; omit to take the default.
  credential_ref = "anthropic-api-key"

  # Bind any integration connections the entry requires, keyed by provider.
  integration_connections = {
    aws = "conn_01hxyz"
  }
}
