resource "agentops_channel" "alerts" {
  channel_provider = "slack"
  connector        = "bot"
  display_name     = "prod-alerts"

  # app_token is write-only and never returned by the API.
  app_token = var.slack_bot_token

  config = jsonencode({
    default_channel = "C0123456789"
  })

  labels = {
    env = "production"
  }
}
