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

variable "agent_id" {
  description = "ID of the agent the resources target."
  type        = string
}

variable "openai_api_key" {
  description = "Secret value stored in the openai credential."
  type        = string
  sensitive   = true
}
