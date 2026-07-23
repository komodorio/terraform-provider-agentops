# Connect Grafana / AlertManager to the incident webhook

Point a real alert source at the webhook the [incident pipeline](../README.md) provisions, so firing
alerts become AgentOps incidents. Two tracks are covered — **Prometheus AlertManager** and **Grafana
unified alerting** — plus the sample configs in this folder:

```
alerting/
  README.md                  this guide
  alertmanager.yml           Prometheus AlertManager receiver + route
  grafana-contact-point.yaml Grafana provisioning: webhook contact point + policy
```

## 1. Get the endpoint and token

Both come from the Terraform outputs (the token is sensitive, so fetch it explicitly):

```bash
terraform output -raw incident_webhook_url     # https://<host>/api/v1/webhooks/endpoint/<trigger_id>
terraform output -raw incident_webhook_token   # the secret token
```

The URL is **per-trigger**: `POST {host}/api/v1/webhooks/endpoint/{trigger_id}`. The token is
returned only at create/rotation, so capture it now.

## 2. How auth works (read before wiring)

This example's trigger uses the default **token** webhook type, which accepts the token **either** way:

- `X-Webhook-Token: <token>` — simplest for `curl` or a proxy you control.
- `Authorization: Bearer <token>` — the static validator's fallback, and what Grafana and
  AlertManager can send **natively**. Use this for both integrations below.

`401` means the header/token is wrong.

## 3. The label contract (so alerts actually match)

The pipeline's `routing_rule` is `environment=staging`, `severity=critical`. An alert is only
turned into an incident if its labels satisfy that rule — a non-matching alert returns `202` but
creates nothing. Make sure the alerts you route carry:

- `severity: critical`
- an environment label of `staging` (the handler recognizes `env` / `environment`)

AlertManager envelopes are auto-unwrapped (`{"alerts":[…]}`), `resolved` alerts are accepted but
skipped, and `commonLabels`/`commonAnnotations` are merged into each alert. Severity maps
case-insensitively: `critical`/`P1` → SEV-1, `high`/`P2` → SEV-2, `warning`/`P3` → SEV-3, and so on.

## 4a. Prometheus AlertManager

Add a receiver that POSTs to the endpoint and a route that sends critical/staging alerts to it.
See [`alertmanager.yml`](alertmanager.yml) for a complete minimal file; the essential part:

```yaml
receivers:
  - name: agentops-incidents
    webhook_configs:
      - url: https://<AGENTOPS_HOST>/api/v1/webhooks/endpoint/<TRIGGER_ID>
        send_resolved: true
        http_config:
          authorization:
            type: Bearer
            credentials: <WEBHOOK_TOKEN>   # from `terraform output -raw incident_webhook_token`
```

Route matching alerts to it:

```yaml
route:
  routes:
    - matchers: [ 'severity="critical"', 'env="staging"' ]
      receiver: agentops-incidents
```

Reload AlertManager (`SIGHUP` or `POST /-/reload`). `send_resolved: true` is fine — resolved alerts
are accepted and skipped, and the alert `fingerprint` AlertManager already sends drives dedup.

> Prefer not to embed the token in the config? Front the endpoint with a proxy that injects the
> `X-Webhook-Token` header, and point `url` at the proxy.

## 4b. Grafana unified alerting

Create a **Webhook** contact point pointing at the endpoint, using Bearer auth. Via the UI:
**Alerting → Contact points → Add → Webhook**, set the URL, and under **Optional settings** set
*Authorization Header* → Scheme `Bearer`, Credentials `<WEBHOOK_TOKEN>`. Then add a **notification
policy** that routes `severity = critical` and `env = staging` to this contact point.

Or provision it as code — see [`grafana-contact-point.yaml`](grafana-contact-point.yaml):

```yaml
apiVersion: 1
contactPoints:
  - orgId: 1
    name: agentops-incidents
    receivers:
      - uid: agentops-incidents
        type: webhook
        settings:
          url: https://<AGENTOPS_HOST>/api/v1/webhooks/endpoint/<TRIGGER_ID>
          httpMethod: POST
          authorization_scheme: Bearer
          authorization_credentials: <WEBHOOK_TOKEN>
```

Grafana sends its own alert payload (provider `generic` parses it). Keep the default JSON body.

## 5. Verify

1. Fire a test alert (or use the ready-made `curl` from the root example's `how_to_test` output).
2. Expect **HTTP 202** from the endpoint.
3. Open `/incidents` in the AgentOps UI — a new incident should appear and move through
   `open → triaging → triaged` as the orchestrator runs.

**Troubleshooting**

| Symptom | Cause |
| --- | --- |
| `401` | Wrong token, or the header/scheme isn't `X-Webhook-Token` / `Authorization: Bearer`. |
| `202` but no incident | Labels didn't match `routing_rule` (`severity=critical`, env `staging`). |
| `200` with `throttled: true` | Rate limit hit — expected under a burst; not an error. |
| Incident updates but no new run | Same `fingerprint` re-fired — only the first fire dispatches a run. |
