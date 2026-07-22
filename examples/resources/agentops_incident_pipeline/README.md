# Self-contained incident pipeline

Stands up a complete incident-response pipeline from scratch — no pre-existing
IDs required:

- an **orchestrator** deployed from the worker catalog,
- two **specialist** hosted agents (database and networking) sharing one LLM
  credential,
- a **webhook trigger** that alerts POST into, and
- the **incident pipeline** that routes matching alerts (environment=production,
  severity=critical) to the orchestrator, which delegates to the specialists.

Each agent's tenant is derived from your account server-side.

```shell
export AGENTOPS_API_KEY="…"
terraform init
terraform apply -var specialist_llm_key="sk-ant-…"
```

Set `-var endpoint=https://staging.agentops.komodor.com` (or `AGENTOPS_ENDPOINT`)
to target staging or a self-hosted deployment.

## Testing it

After `apply`, the `how_to_test` output prints a ready-to-run `curl` that fires a
test alert matching the routing rule. In short:

```shell
TOKEN=$(terraform output -raw incident_webhook_token)
curl -sS -X POST "$(terraform output -raw incident_webhook_url)" \
  -H "X-Webhook-Token: $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "status": "firing",
    "labels": { "alertname": "HighErrorRate", "env": "production", "severity": "critical" },
    "annotations": { "description": "Error rate exceeded threshold on checkout-service" },
    "fingerprint": "test-'"$(date +%s)"'"
  }'
```

Expect HTTP 202, then check `/incidents` in the AgentOps UI for the new incident.
Use a fresh `fingerprint` each time — re-firing the same one updates the incident
but does not dispatch a new orchestrator run.
