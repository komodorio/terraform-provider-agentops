# Configure the AgentOps provider. The API key and endpoint can also be
# supplied via the AGENTOPS_API_KEY and AGENTOPS_ENDPOINT environment variables.
provider "agentops" {
  # endpoint = "https://agentops.komodor.com" # default; use staging/self-hosted as needed
  api_key = var.agentops_api_key
}
