# Provider setup and inputs for the self-contained incident pipeline example.
# The resources live in resource.tf; the handy outputs live in outputs.tf.

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

variable "api_key" {
  description = "AgentOps API key (a PAT or Service Account token). Create one at agentops.komodor.com → Settings → API Key. Prefer the AGENTOPS_API_KEY environment variable."
  type        = string
  sensitive   = true
  default     = null
}

variable "endpoint" {
  description = "AgentOps control-plane base URL. Leave null to use the default."
  type        = string
  default     = null
}

variable "specialist_llm_key" {
  description = "LLM API key for the specialist agents. Must match the provider of their `model` — an Anthropic key for the default claude-sonnet-5. Using a different provider? Set `model` on the specialists accordingly and pass that provider's key here. Reusing agents you already have deployed? You don't need this — bind their runtime_agent_ids in specialist_bindings instead (see the README)."
  type        = string
  sensitive   = true
}
