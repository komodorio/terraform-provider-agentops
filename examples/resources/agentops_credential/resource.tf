resource "agentops_credential" "openai" {
  name        = "openai-api-key"
  value       = var.openai_api_key # write-only; never read back
  description = "OpenAI API key for agents"
  labels = {
    team = "platform"
  }
}
