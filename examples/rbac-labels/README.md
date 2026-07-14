# RBAC scenario — attribute-based access with labels

Grants normally target a resource by id. This scenario targets resources by
their **labels** instead, using a grant `selector`:

- **labelled resources**: two MCP gateway servers tagged `env = prod|staging`
  and `team = sre`.
- **label-scoped grant**: an operator role granted over `resource_id = "*"`
  (all servers) but narrowed by `selector = { env = "prod", team = "sre" }`, so
  only the prod-labelled server is in scope.

Because the grant follows labels rather than ids, re-labelling a server moves it
in or out of scope automatically — no grant change required.

```shell
export AGENTOPS_API_KEY="…"
terraform init
terraform apply
```

The `selector` is a free-form JSON attribute matcher, and `resource_type` should
be a type reported by the `agentops_resource_types` data source. Set
`-var endpoint=https://staging.agentops.komodor.com` (or `AGENTOPS_ENDPOINT`) to
target staging or a self-hosted deployment.
