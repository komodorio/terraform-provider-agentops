# RBAC scenario — resource patterns

Shows how to authorize a subject against *many* resources at once instead of one
at a time, and both ways to issue a grant:

- **wildcard pattern**: `resource_id = "*"` targets every resource of a type
  (here, every agent) — current and future. Set `resource_id` to a concrete id
  to scope a grant to a single resource instead.
- **direct capability grant** (`grant_kind = "capability"`): binds one
  capability straight to the subject, no role required.
- **role grant** (`grant_kind = "role"`): applies a whole role over the same
  wildcard pattern.

```shell
export AGENTOPS_API_KEY="…"
terraform init
terraform apply
```

Set `-var endpoint=https://staging.agentops.komodor.com` (or `AGENTOPS_ENDPOINT`)
to target staging or a self-hosted deployment.
