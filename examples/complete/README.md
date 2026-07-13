# End-to-end example

Wires the AgentOps config plane together: an authorization policy → role →
service account → grant, an API key and a credential (with an agent binding), a
knowledge base (with an agent grant), and automation (workflow, trigger,
schedule), plus read-only catalog/capabilities lookups.

```shell
export AGENTOPS_API_KEY="…"
terraform init
terraform apply \
  -var agent_id=agent_01hxyz \
  -var openai_api_key="sk-…"
```

Set `-var endpoint=https://staging.agentops.komodor.com` (or `AGENTOPS_ENDPOINT`)
to target staging or a self-hosted deployment.
