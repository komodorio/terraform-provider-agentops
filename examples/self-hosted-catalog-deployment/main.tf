# Self-hosted worker-catalog deploy — register a curated worker with the control
# plane, then run it yourself with the agentops-agent-base Helm chart.
#
# Prerequisites:
#   1. Build the provider locally:
#        cd <repo-root> && go install ./...
#   2. Export credentials — this endpoint requires a USER-bound API key (a PAT);
#      a service-account token is rejected with 403:
#        export AGENTOPS_ENDPOINT=https://staging.agentops.komodor.com
#        export AGENTOPS_API_KEY=<your staging personal API key>
#   3. Ensure a credential named ANTHROPIC_API_KEY exists on the account.
#   4. Run:
#        terraform plan
#        terraform apply -auto-approve
#   5. Install the worker into your cluster with the emitted token (see the
#      helm_install_hint output), then:
#        terraform destroy -auto-approve

terraform {
  required_providers {
    agentops = {
      source = "komodorio/agentops"
    }
    kubernetes = {
      source = "hashicorp/kubernetes"
    }
  }
}

provider "agentops" {}

provider "kubernetes" {
  config_path = "~/.kube/config"
}

# ── Catalog ──────────────────────────────────────────────────────────────────

data "agentops_worker_catalog" "all" {}

# ── Self-hosted deploy ───────────────────────────────────────────────────────

resource "agentops_self_hosted_catalog_deployment" "k8s_troubleshooter" {
  catalog_id     = "k8s-troubleshooter"
  agent_id       = "tf-smoke-k8s-troubleshooter"
  display_name   = "TF Smoke K8s Troubleshooter (self-hosted)"
  credential_ref = "ANTHROPIC_API_KEY"
}

# ── Hand the once-only token to the cluster ──────────────────────────────────

resource "kubernetes_secret" "worker_token" {
  metadata {
    name      = "tf-smoke-k8s-troubleshooter-token"
    namespace = "agentops"
  }

  data = {
    AGENTOPS_WORKER_TOKEN = agentops_self_hosted_catalog_deployment.k8s_troubleshooter.token
  }
}

# ── Outputs ──────────────────────────────────────────────────────────────────

output "agent_id" {
  value = agentops_self_hosted_catalog_deployment.k8s_troubleshooter.agent_id
}

output "worker_token" {
  value     = agentops_self_hosted_catalog_deployment.k8s_troubleshooter.token
  sensitive = true
}

output "worker_token_hint" {
  value = agentops_self_hosted_catalog_deployment.k8s_troubleshooter.worker_token_hint
}

# Stays draft/offline until the chart is installed and the worker heartbeats.
output "status" {
  value = agentops_self_hosted_catalog_deployment.k8s_troubleshooter.status
}

# This endpoint returns no rendered values YAML — the image comes from the
# catalog entry and the token is supplied by the secret above.
output "helm_install_hint" {
  value = <<-EOT
    helm upgrade --install ${agentops_self_hosted_catalog_deployment.k8s_troubleshooter.agent_id} \
      oci://ghcr.io/komodorio/helm-charts/agentops-agent-base \
      --namespace agentops --create-namespace \
      --set agent.agentId=${agentops_self_hosted_catalog_deployment.k8s_troubleshooter.agent_id} \
      --set agent.existingTokenSecret=${kubernetes_secret.worker_token.metadata[0].name}
  EOT
}
