## 0.1.0 (Unreleased)

FEATURES:

* resource/agentops_hosted_agent: Add `agents` attribute for delivering Claude Code subagents (`.claude/agents/*.md`) to a hosted agent, mirroring `skills`.

BUG FIXES:

* resource/agentops_hosted_agent, resource/agentops_worker_catalog_deployment: `wait_for_online` now ends the wait as soon as a deploy fails, reporting the control plane's reason. The hosted-agent record's status only tracks worker heartbeats, so a provision that never produced a worker read as `draft` and the apply ran out the full `wait_timeout` before blaming the timeout.
* resource/agentops_hosted_agent, resource/agentops_worker_catalog_deployment: Deleting an agent that deployed successfully now archives it first, then waits for the archive to finish before deleting. On an account with hosted lifecycle enabled the API refuses the delete with `409 archive the agent before deleting it`, which left `terraform destroy` unable to remove what it had created. Deleting while the archive is still deploying takes a different server-side path that drops the record without revoking the agent's worker token, so the destroy fails rather than deleting early.
