# Reviewer agents (including built-ins) available to the account.
data "agentops_reviewers" "all" {}

resource "agentops_review_workflow" "backend" {
  name               = "backend-pr-reviews"
  status             = "active"
  base_branch_filter = "main"

  reviewer_agent_ids = [for r in data.agentops_reviewers.all.reviewers : r.agent_id if r.is_builtin]

  repos = [
    {
      repo_owner = "komodorio"
      repo_name  = "mono"
    },
    {
      repo_owner = "komodorio"
      repo_name  = "infra"
    },
  ]
}
