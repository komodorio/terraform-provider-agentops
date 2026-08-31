# Deploy a curated worker from the catalog into your own cluster. The control
# plane registers the agent and mints its worker token; you run the container.
# Browse available entries (and their IDs / required credentials) via the
# agentops_worker_catalog data source.
#
# The endpoint requires a user-bound API key (a PAT) — a service-account token
# is rejected.
resource "agentops_self_hosted_catalog_deployment" "k8s_troubleshooter" {
  catalog_id   = "k8s-troubleshooter"
  agent_id     = "prod-k8s-troubleshooter"
  display_name = "Prod K8s Troubleshooter (self-hosted)"

  # LLM credential (by name) from the entry's allowed set; omit to take the default.
  credential_ref = "anthropic-api-key"

  # Bind any integration connections the entry requires, keyed by provider.
  integration_connections = {
    aws = "conn_01hxyz"
  }
}

# The token is returned once, at deploy time. Feed it to the worker as
# AGENTOPS_WORKER_TOKEN — here via a Kubernetes secret the agentops-agent-base
# chart consumes.
resource "kubernetes_secret" "worker_token" {
  metadata {
    name      = "prod-k8s-troubleshooter-token"
    namespace = "agentops"
  }

  data = {
    AGENTOPS_WORKER_TOKEN = agentops_self_hosted_catalog_deployment.k8s_troubleshooter.token
  }
}
