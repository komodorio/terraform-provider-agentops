variable "api_key" {
  description = "AgentOps API key. Prefer the AGENTOPS_API_KEY environment variable."
  type        = string
  sensitive   = true
  default     = null
}

variable "endpoint" {
  description = "AgentOps control-plane base URL. Leave null to use the default."
  type        = string
  default     = null
}

variable "service_account_id" {
  description = "Service account the CI API key is bound to."
  type        = string
}

variable "agent_id" {
  description = "ID of the agent the trigger invokes."
  type        = string
}
