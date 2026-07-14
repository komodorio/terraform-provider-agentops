# Automation scenario

Gives one agent a run-time credential, then drives it three ways:

- **credential (+ binding)**: an OpenAI key bound to the agent.
- **workflow**: an on-demand, multi-step `incident-triage` run.
- **trigger**: a webhook that invokes the agent; the invocation token is
  exported once at create time.
- **schedule**: a nightly cron run at 02:00 UTC.

```shell
export AGENTOPS_API_KEY="…"
terraform init
terraform apply \
  -var agent_id=agent_01hxyz \
  -var openai_api_key="sk-…"
```

Set `-var endpoint=https://staging.agentops.komodor.com` (or `AGENTOPS_ENDPOINT`)
to target staging or a self-hosted deployment.
