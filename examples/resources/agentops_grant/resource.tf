resource "agentops_grant" "sre_deployer" {
  grant_kind    = "role"
  role_id       = agentops_role.deployer.id
  resource_type = "agent"
  resource_id   = "*"

  subject = jsonencode({
    id   = "prn_01hxyz"
    kind = "principal"
  })
}
