# Komodor AgentOps Terraform Provider

Manage [Komodor AgentOps](https://agentops.komodor.com) config-plane resources as
code. The provider talks to the AgentOps control-plane API and is built on the
[Terraform Plugin Framework](https://github.com/hashicorp/terraform-plugin-framework).

Provider address: `registry.terraform.io/komodorio/agentops`.

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.25 (only to build the provider from source)

## Using the Provider

```hcl
terraform {
  required_providers {
    agentops = {
      source = "komodorio/agentops"
    }
  }
}

provider "agentops" {
  # endpoint defaults to https://agentops.komodor.com.
  # Use https://staging.agentops.komodor.com for staging, or your own URL
  # for a self-hosted deployment.
  # endpoint = "https://agentops.komodor.com"

  api_key = var.agentops_api_key
}

resource "agentops_trigger" "deploy" {
  name        = "deploy-webhook"
  target_id   = "agent_01hxyz"
  target_type = "agent"
}
```

### Authentication

The provider authenticates with an AgentOps API key sent as a Bearer token. Both
settings can come from the configuration block or the environment:

| Setting  | Argument   | Environment variable | Default                        |
| -------- | ---------- | -------------------- | ------------------------------ |
| API key  | `api_key`  | `AGENTOPS_API_KEY`   | — (required)                   |
| Endpoint | `endpoint` | `AGENTOPS_ENDPOINT`  | `https://agentops.komodor.com` |

```shell
export AGENTOPS_API_KEY="…"
export AGENTOPS_ENDPOINT="https://staging.agentops.komodor.com" # optional
```

### Resources and data sources

| Type        | Name                              | Description                                       |
| ----------- | --------------------------------- | ------------------------------------------------- |
| Resource    | `agentops_trigger`                | Webhook trigger that invokes an agent or workflow |
| Resource    | `agentops_api_key`                | API key (create/delete only; secret shown once)   |
| Resource    | `agentops_service_account`        | Service account (create/delete only)              |
| Resource    | `agentops_schedule`               | Cron schedule that runs an agent                  |
| Resource    | `agentops_credential`             | Stored credential with a write-only value         |
| Resource    | `agentops_workflow`               | Multi-step agent workflow                         |
| Resource    | `agentops_integration_connection` | Connection to an external integration provider    |
| Resource    | `agentops_knowledge_base`         | Knowledge base for agents                         |
| Resource    | `agentops_mcp_gateway_server`     | Upstream MCP server registered on the gateway     |
| Resource    | `agentops_mcp_gateway_group`      | Group of MCP gateway servers                      |
| Resource    | `agentops_mcp_gateway_policy`     | MCP gateway access policy                         |
| Data source | `agentops_integration_catalog`    | Integration providers available to the account    |

More resources are added over time. See [`docs/`](./docs) for the full,
generated reference and [`examples/`](./examples) for usage examples.

## Developing the Provider

Requires [Go](https://golang.org/doc/install) >= 1.25.

Build and install into `$GOPATH/bin`:

```shell
make install
```

Run unit tests and the mock-backed acceptance tests:

```shell
make test              # unit tests
make testacc           # acceptance tests (TF_ACC=1); run in-process against a mock API
```

Lint and format:

```shell
make lint
make fmt
```

### The generated client

The typed API client in [`internal/client/gen/`](./internal/client/gen) is
generated from the AgentOps OpenAPI spec with
[`oapi-codegen`](https://github.com/oapi-codegen/oapi-codegen); a small
hand-written layer in [`internal/client/client.go`](./internal/client/client.go)
adds the base URL, Bearer auth, retries, and error handling.

```shell
# Refresh the vendored spec from a local monorepo checkout …
make sync-spec MONOREPO=../agentops
# … or from a running endpoint:
make sync-spec ENDPOINT=https://staging.agentops.komodor.com

# Regenerate the client from the vendored spec:
make generate
```

The vendored `api/openapi.json` is kept as the faithful OpenAPI 3.1 document the
API emits. Because `oapi-codegen` cannot consume 3.1 directly, `make generate`
transiently down-converts it to a 3.0 subset (via `internal/specdowngrade`)
before generating; the committed client stays in sync with the spec and CI fails
on any drift.

### Documentation

Docs under [`docs/`](./docs) are generated from the resource schemas and the
files in [`examples/`](./examples):

```shell
make docs
```

## Releasing and publishing

The provider is published as `komodorio/agentops`. Releases are built and signed
by GoReleaser and triggered by pushing a `v*` tag. See [`RELEASING.md`](./RELEASING.md)
for how to cut a release, the required signing secrets, publishing to the public
Terraform Registry, and consuming the provider privately (filesystem mirror or
dev overrides) in the meantime.
