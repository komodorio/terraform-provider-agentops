# RBAC scenario — name globs

Scopes authorization to a set of resources by **name glob**. The glob does not
live in `resource_id` (which is exact-or-`"*"` only) — it goes in the grant
`selector` under the reserved `name_glob` key, with `resource_id` left as `"*"`:

```hcl
resource_id = "*"
selector    = jsonencode({ name_glob = "agent_prod-*" })
```

`*` is the only wildcard (every other character is literal), so
`"agent_prod-*"` matches `agent_prod-checkout` but not `agent_dev-sandbox`. The
invariant is `name_glob` ⇔ `resource_id == "*"`: a `name_glob` is only valid
with a `"*"` id.

The example builds a `prod-agent-operator` policy whose grants are name-glob
scoped, bundles it into a role, and binds that role to a service account — the
binding grant glob-scoped the same way. It also shows a standalone capability
grant using the same `name_glob`, without a policy or role.

```shell
export AGENTOPS_API_KEY="…"
terraform init
terraform apply
```

`name_glob` is identity matching; ABAC *label* selectors use other keys — see
the `rbac-labels` example. Set `-var endpoint=https://staging.agentops.komodor.com`
(or `AGENTOPS_ENDPOINT`) to target staging or a self-hosted deployment.
