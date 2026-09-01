# Skills Registry scenario

Authors a versioned skill, attaches it to an agent, and shows how to publish new
versions of the skill's content over time.

- **skill** (`deploy-runbook`): a Markdown document whose body lives in
  [`skills/deploy-runbook.md`](./skills/deploy-runbook.md), tagged `ops`/`deploy`
  and scoped with a `team` label.
- **binding**: attaches the skill to an agent, floating on the latest published
  version.

Requires the account-level `skills_registry` feature — mutations return `404`
while it is disabled.

## Apply

```shell
export AGENTOPS_API_KEY="…"
terraform init
terraform apply -var agent_id=agent_01hxyz
```

Creating the skill with `content` set publishes **version 1**. Check it:

```shell
terraform output content_version   # => 1
```

## Publishing a new version

The skill's content is versioned. To publish a new version, edit the Markdown
body and re-apply — the provider detects the changed `content` and publishes a
new version through `POST /api/v1/skills/{id}/versions`, bumping
`content_version`:

```shell
$EDITOR skills/deploy-runbook.md      # change the body
terraform apply -var agent_id=agent_01hxyz
terraform output content_version      # => 2
```

Metadata edits (`name`, `description`, `tags`, `labels`) are applied in place and
do **not** create a new version — only a change to `content` does. If the body's
YAML front-matter carries a `description`, the control plane derives the skill's
`description` from it.

Because the binding omits `pin_version_id`, the agent floats on the latest
version and picks up each new one automatically. To freeze an agent on a specific
version, set `pin_version_id` on `agentops_agent_skill` to that version's id.

Set `-var endpoint=https://staging.agentops.komodor.com` (or `AGENTOPS_ENDPOINT`)
to target staging or a self-hosted deployment.
