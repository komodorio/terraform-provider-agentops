---
name: terraform-incident-pipeline
description: Use when running or modifying the AgentOps incident-pipeline Terraform example in examples/resources/agentops_incident_pipeline — applying it, adapting the routing rule / specialists / alert source / model, targeting a non-prod control plane, or debugging why alerts create an incident but no orchestrator run.
---

# Run and modify the incident pipeline

## Mental model (read first)

- The example at `examples/resources/agentops_incident_pipeline/` stands up a whole AgentOps
  incident-response pipeline from scratch — an orchestrator (from the worker catalog), two specialist
  hosted agents sharing one LLM credential, a standalone webhook trigger, and the
  `agentops_incident_pipeline` that wires them together. No pre-existing IDs are needed.
- **The resource shape is fixed by the provider; you adapt values.** The runnable config lives in
  `resource.tf`; provider + inputs in `main.tf`; helper outputs in `outputs.tf`.
- **An alert only becomes an incident if it matches the `routing_rule`.** The orchestrator run is
  dispatched only on the *first* fire of a given fingerprint.

## Prerequisites

- Provider `komodorio/agentops` (pulled by `terraform init`; no version pinned).
- An **AgentOps API key** as `AGENTOPS_API_KEY` (or `-var api_key=…`). Get one at
  agentops.komodor.com → log in → **Settings → API Key** → create a **PAT** or a **Service Account**
  token. Target a non-prod control plane with `-var endpoint=https://staging.agentops.komodor.com` or
  `AGENTOPS_ENDPOINT`.
- A `specialist_llm_key` matching the specialists' `model` — `credential_ref` resolves the credential
  **by name**, so the key must match the model's provider. The default `claude-sonnet-5` needs an
  **Anthropic** key; for another provider, set `model` accordingly and pass its key. **Reusing agents
  you already have deployed?** You don't need this key — bind their IDs in `specialist_bindings`
  instead (find IDs in the UI: agentops.komodor.com → **Agents** → open the agent → copy its ID).

## Steps

### 1. Apply
```bash
cd examples/resources/agentops_incident_pipeline
export AGENTOPS_API_KEY="…"
terraform init
terraform apply -var specialist_llm_key="sk-ant-…"
```
`terraform output how_to_test` prints a copy-paste `curl` that fires a matching test alert.

### 2. Modify — the common changes
All in `resource.tf` unless noted:

- **Routing rule** (`agentops_incident_pipeline.incidents.routing_rule`): change `environment` / `severity`,
  or add `service`, `tags` (map), `route_all = true`, or `missing_field_default`. This governs which
  alerts become incidents — keep it in sync with the labels your alert source sends.
- **Alert source** (`alert_source`): `provider = "generic"` (AlertManager / Grafana / any webhook) or
  `"datadog"`; `monitor_mode = "create_catchall"` or `"link_existing"` (+ `external_monitor_id`).
- **Specialists** (`specialist_bindings[]`): add/remove `{ agent_id, role, enabled }`. `role` is
  free-text; add a new `agentops_hosted_agent` (or reuse a deployed Fleet agent) and bind its
  `runtime_agent_id`. Built-in roles include `datadog-investigator`, `aws-investigator`,
  `komodor-investigator`.
- **Model / instructions**: edit each `agentops_hosted_agent`. If you change `model` to a non-Anthropic
  provider, point `credential_ref` at a matching credential.
- **Slack summary**: add `delivery_config = { slack = { channel_id = "…", enabled = true } }`.

### 3. Re-apply and re-test
`terraform apply`, then fire a fresh alert (new `fingerprint`) and watch `/incidents` in the UI.

### 4. Destroy
`terraform destroy` (it pauses the pipeline before deleting). To adopt an existing pipeline instead,
`terraform import agentops_incident_pipeline.incidents <ipl_…>`.

## Invariants

- **Bind `runtime_agent_id`, never the resource `id`.** In `orchestrator_binding` and
  `specialist_bindings`, use `<agent>.runtime_agent_id` (dispatcher-resolvable). Binding `.id` (the
  record PK) lets alerts create an incident but silently never dispatches the orchestrator run — the
  most common failure mode.
- **Lifecycle: draft → active → paused, and most edits force replacement.** Only `status` changes in
  place; every config-bearing attribute is `RequiresReplace` because the API only accepts config edits
  while draft, and a published pipeline can't return to draft. Expect replacement on config changes.
- **An endpoint must be linked before activation** — the trigger is created standalone and referenced
  by `trigger_id`; the provider links it, then activates.
- The webhook **token is returned only at create/rotation** — `terraform output -raw
  incident_webhook_token`. To hand alerting off, see the
  [`connect-alerting-webhook`](../connect-alerting-webhook/SKILL.md) skill.
