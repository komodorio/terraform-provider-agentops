# End-to-end example

Discovers available integrations, mints an API key bound to a service account,
and registers a webhook trigger for an agent.

```shell
export AGENTOPS_API_KEY="…"
terraform init
terraform apply -var service_account_id=sa_01hxyz -var agent_id=agent_01hxyz
```

Set `-var endpoint=https://staging.agentops.komodor.com` (or `AGENTOPS_ENDPOINT`)
to target staging or a self-hosted deployment.
