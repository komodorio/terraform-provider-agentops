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
  description = "ID of the agent the CI service account is scoped to."
  type        = string
}

variable "member_email" {
  description = "Email of the human member to invite and grant the operator role."
  type        = string
}
