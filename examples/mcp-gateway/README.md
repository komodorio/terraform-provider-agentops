# MCP gateway scenario

Fronts two upstream MCP servers behind the AgentOps gateway, bundles them into a
group agents can attach to, and gates tool access with a default-deny policy:

- **servers**: `docs-mcp` (search tools only) and `issues-mcp`, each
  authenticated with a bearer token resolved from an environment secret at
  connect time.
- **group**: `core-tools` bundles both servers.
- **policy**: default-deny with an explicit tool allowlist.

```shell
export AGENTOPS_API_KEY="…"
terraform init
terraform apply
```

The `$${env:DOCS_MCP_TOKEN}` / `$${env:ISSUES_MCP_TOKEN}` references in
`static_headers` are resolved by the gateway at connect time — the literal
secret is never stored in Terraform state.

Set `-var endpoint=https://staging.agentops.komodor.com` (or `AGENTOPS_ENDPOINT`)
to target staging or a self-hosted deployment.
