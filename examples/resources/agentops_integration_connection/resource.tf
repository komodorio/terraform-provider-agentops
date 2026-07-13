resource "agentops_integration_connection" "github" {
  provider_key = "github"
  display_name = "GitHub (org)"

  credentials = {
    token = var.github_token # write-only
  }

  metadata = jsonencode({ org = "komodorio" })
}
