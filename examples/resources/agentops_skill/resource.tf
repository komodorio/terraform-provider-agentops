# An authored skill: a versioned Markdown document agents can be attached to.
# Setting `content` on create publishes version 1; editing it and re-applying
# publishes a new version and bumps the computed `content_version`.
resource "agentops_skill" "deploy_runbook" {
  name        = "deploy-runbook"
  description = "How to safely roll out a service"

  content = <<-EOT
    # Deploy runbook

    Roll out changes in stages, verifying health before widening the blast radius.

    1. Deploy to the canary and watch error rate for 10 minutes.
    2. If healthy, roll out to 25%, 50%, then 100%.
    3. On any regression, roll back to the previous version.
  EOT

  tags = ["ops", "deploy"]
  labels = {
    team = "platform"
  }
}

# For a real skill, keep the body in a file and let a change to it publish a new
# version: content = file("${path.module}/skills/deploy-runbook.md")
