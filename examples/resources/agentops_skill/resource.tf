# An authored skill: a versioned Markdown document agents can be attached to.
resource "agentops_skill" "deploy_runbook" {
  name        = "deploy-runbook"
  description = "How to safely roll out a service"
  content     = file("${path.module}/skills/deploy-runbook.md")
  tags        = ["ops", "deploy"]
  labels = {
    team = "platform"
  }
}
