# RBAC scenario

Builds an authorization model bottom-up and assigns it to two kinds of subjects:

- **policy → role**: a `operate-agents` policy (invoke + read capabilities)
  bundled into an `operator` role.
- **machine identity**: a service account + API key, granted the operator role
  on a single agent.
- **human identity**: a member, granted the operator role on all agents (`*`).

```shell
export AGENTOPS_API_KEY="…"
terraform init
terraform apply \
  -var agent_id=agent_01hxyz \
  -var member_email="on-call@example.com"
```

Set `-var endpoint=https://staging.agentops.komodor.com` (or `AGENTOPS_ENDPOINT`)
to target staging or a self-hosted deployment.
