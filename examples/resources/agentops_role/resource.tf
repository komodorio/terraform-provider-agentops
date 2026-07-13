resource "agentops_role" "deployer" {
  name        = "deployer"
  description = "Can deploy and manage agents"
  policy_ids  = [agentops_policy.deploy.id]
}
