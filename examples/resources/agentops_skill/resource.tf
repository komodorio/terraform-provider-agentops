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

# A skill is a folder, not one file: `content` is SKILL.md and `resources` are the
# siblings it links to, at the same relative paths the body writes. The worker
# materializes the folder where its harness looks for skills, so a link to
# references/rollback.md resolves and scripts/deploy.sh arrives executable.
resource "agentops_skill" "deploy_runbook_folder" {
  name = "deploy-runbook-with-references"

  content = <<-EOT
    # Deploy runbook

    Roll out in stages. For the rollback procedure see [references/rollback.md](references/rollback.md).
    To cut a canary, run `scripts/deploy.sh --canary`.
  EOT

  resources = [
    {
      path    = "references/rollback.md"
      content = file("${path.module}/skills/deploy-runbook/references/rollback.md")
    },
    {
      path       = "scripts/deploy.sh"
      content    = file("${path.module}/skills/deploy-runbook/scripts/deploy.sh")
      executable = true
    },
  ]
}
