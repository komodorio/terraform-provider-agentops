## 0.1.0 (Unreleased)

FEATURES:

* resource/agentops_hosted_agent: Add `agents` attribute for delivering Claude Code subagents (`.claude/agents/*.md`) to a hosted agent, mirroring `skills`.

BUG FIXES:

* resource/agentops_hosted_agent, resource/agentops_worker_catalog_deployment: `wait_for_online` now ends the wait as soon as a deploy fails, reporting the control plane's reason. The hosted-agent record's status only tracks worker heartbeats, so a provision that never produced a worker read as `draft` and the apply ran out the full `wait_timeout` before blaming the timeout.
