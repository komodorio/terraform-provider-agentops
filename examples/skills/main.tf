# Skills Registry scenario: author a versioned skill, attach it to an agent, and
# publish new versions of its content over time.
#
#   skill (versioned Markdown document)
#   -> agent_skill (attaches the skill to an agent, floating on the latest version)
#
# Requires the account-level `skills_registry` feature. Mutations 404 while it is
# disabled — ask Komodor support or your account admin to enable it.

terraform {
  required_providers {
    agentops = {
      source = "komodorio/agentops"
    }
  }
}

provider "agentops" {
  endpoint = var.endpoint # optional; defaults to https://agentops.komodor.com
  api_key  = var.api_key
}

# ── Skill: a versioned Markdown document an agent can be attached to ──────────
# The Markdown body lives in a file next to this stack. Creating the skill with
# `content` set publishes version 1. Editing skills/deploy-runbook.md and running
# `terraform apply` again publishes a NEW version and bumps `content_version` —
# see README.md for the full publish flow.
resource "agentops_skill" "deploy_runbook" {
  name        = "deploy-runbook"
  description = "How to safely roll out a service"
  content     = file("${path.module}/skills/deploy-runbook.md")

  tags = ["ops", "deploy"]

  # ABAC labels scope who can read/manage the skill.
  labels = {
    team = "platform"
  }
}

# ── Binding: attach the skill to an agent ────────────────────────────────────
# `pin_version` is omitted, so the binding floats on the latest published
# version: every new version published above is picked up automatically. Set it
# to a published version number (e.g. `pin_version = 1`) to freeze the agent on
# that version instead; the resolved version id is exposed as `pinned_version_id`.
resource "agentops_agent_skill" "runbook_to_agent" {
  agent_id = var.agent_id
  skill_id = agentops_skill.deploy_runbook.id
}
