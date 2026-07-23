---
name: connect-alerting-webhook
description: Use when connecting an external alert source (Prometheus AlertManager or Grafana unified alerting) to the AgentOps incident-pipeline webhook — resolving the endpoint + token, choosing the auth header, making alert labels satisfy the routing rule, and verifying alerts turn into incidents.
---

# Connect an alert source to the incident webhook

## Mental model (read first)

- The incident pipeline (see the [`terraform-incident-pipeline`](../terraform-incident-pipeline/SKILL.md)
  skill) exposes one **per-trigger** webhook: `POST {host}/api/v1/webhooks/endpoint/{trigger_id}`.
- **The token in the header is the auth** — there's no account scoping. Guard it.
- **An alert only becomes an incident if its labels match the pipeline's `routing_rule`.** Wiring the
  transport is half the job; matching the labels is the other half.
- Full walkthrough + copy-paste samples live in
  `examples/resources/agentops_incident_pipeline/alerting/` (`README.md`, `alertmanager.yml`,
  `grafana-contact-point.yaml`).

## Steps

### 1. Resolve the endpoint and token
```bash
cd examples/resources/agentops_incident_pipeline
terraform output -raw incident_webhook_url     # …/api/v1/webhooks/endpoint/<trigger_id>
terraform output -raw incident_webhook_token   # sensitive
```

### 2. Choose the auth header
The default trigger (`token` type) accepts the token **either** way:
- `X-Webhook-Token: <token>` — for `curl` or a proxy you control.
- `Authorization: Bearer <token>` — the static validator's fallback; what AlertManager and Grafana
  send natively. **Use Bearer for both integrations.**

### 3. Wire the source
- **AlertManager**: add a `webhook_configs` receiver with
  `http_config.authorization: { type: Bearer, credentials: <token> }`, `send_resolved: true`, and a
  `route` matcher that selects the alerts to forward. Copy `alerting/alertmanager.yml`; reload with
  `SIGHUP` or `POST /-/reload`.
- **Grafana**: create a **Webhook** contact point (UI: Alerting → Contact points → Webhook, set URL +
  Authorization header scheme `Bearer` / credentials `<token>`), then a notification policy routing the
  right alerts to it. Or provision `alerting/grafana-contact-point.yaml`.

### 4. Satisfy the routing rule
This example routes `environment=staging`, `severity=critical`. The forwarded alerts must carry:
- `severity: critical`
- an environment label of `staging` (the handler recognizes `env` / `environment`).

If you changed the `routing_rule` in Terraform, match the labels to it instead. Severity maps
case-insensitively (`critical`/`P1` → SEV-1, `warning`/`P3` → SEV-3, …). AlertManager envelopes are
auto-unwrapped, `resolved` alerts skipped, `commonLabels` merged.

### 5. Verify
Fire a test alert (or the root example's `how_to_test` `curl`). Expect **HTTP 202**, then check
`/incidents` in the AgentOps UI for a new incident moving `open → triaging → triaged`.

## Invariants

- **URL is per-trigger** (`/api/v1/webhooks/endpoint/{trigger_id}`), not per-pipeline. An older
  `/webhooks/incident-pipelines/{pipeline_id}` form is stale — do not use it.
- **`202` but no incident = labels didn't match the routing rule.** `401` = bad token/header.
  `200 {"throttled": true}` = rate limit (expected under bursts, not an error).
- **Dedup is by fingerprint; only the first fire dispatches a run.** Re-firing the same fingerprint
  updates the incident but starts no new orchestrator run — use a fresh fingerprint to trigger again.
- Never commit the token. Fetch it with `terraform output -raw incident_webhook_token`; to avoid
  embedding it, front the endpoint with a proxy that injects the header.
