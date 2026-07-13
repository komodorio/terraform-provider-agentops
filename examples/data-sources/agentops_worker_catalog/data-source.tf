# Worker/agent templates available to deploy.
data "agentops_worker_catalog" "all" {}

output "ready_workers" {
  value = [for e in data.agentops_worker_catalog.all.entries : e.name if e.ready]
}
