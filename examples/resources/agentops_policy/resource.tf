resource "agentops_policy" "deploy" {
  name        = "deploy-policy"
  description = "Grants deploy capabilities"

  # grants is a free-form JSON array of grant definitions.
  grants = jsonencode([
    { capability = "agents.deploy", resource_type = "agent" },
  ])
}
