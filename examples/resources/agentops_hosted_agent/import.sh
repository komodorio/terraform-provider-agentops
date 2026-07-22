# Hosted agents are imported as "customer/agent_id". Write-only spec fields
# (instructions, model, skills, ...) must be supplied in configuration afterwards.
terraform import agentops_hosted_agent.triage acme/incident-triage
