# Releasing

The provider is released with [GoReleaser](https://goreleaser.com) via the
[`Release`](.github/workflows/release.yml) workflow, producing cross-platform
binaries, a checksums file, and a GPG detached signature in the layout the
Terraform Registry expects.

## Cutting a release

1. Make sure `master` is green and up to date.
2. Tag the commit and push the tag:

   ```shell
   git tag v0.1.0
   git push origin v0.1.0
   ```

3. The `Release` workflow triggers on `v*` tags, runs `goreleaser release
   --clean`, and publishes a GitHub Release with the built artifacts.

Validate the config locally before tagging:

```shell
goreleaser check                                    # lint .goreleaser.yml
goreleaser build --snapshot --clean --single-target # smoke-build one target
```

### Required repository secrets

The release job signs the checksums with GPG. Configure these under
**Settings → Secrets and variables → Actions**:

| Secret            | Purpose                                  |
| ----------------- | ---------------------------------------- |
| `GPG_PRIVATE_KEY` | ASCII-armored private signing key        |
| `PASSPHRASE`      | Passphrase for that key                  |

`GITHUB_TOKEN` is provided automatically.

## Publishing to the Terraform Registry (public)

When ready to publish publicly under the `komodorio` namespace:

1. Add the GPG **public** key to the Registry (Organization → Settings → GPG
   keys).
2. Connect this repository on <https://registry.terraform.io> and let it index
   the tagged releases. The registry reads
   [`terraform-registry-manifest.json`](terraform-registry-manifest.json)
   (protocol `6.0`).

Consumers then just declare the provider:

```hcl
terraform {
  required_providers {
    agentops = {
      source  = "komodorio/agentops"
      version = "~> 0.1"
    }
  }
}
```

## Consuming privately (before public registry publish)

While the repository is private, consumers install the provider without the
public registry using one of the following.

### Filesystem mirror

Download the release zip for your platform from the GitHub Release and unzip the
binary into the local plugin mirror:

```
~/.terraform.d/plugins/registry.terraform.io/komodorio/agentops/<version>/<os>_<arch>/terraform-provider-agentops_v<version>
```

Then reference the provider normally (`source = "komodorio/agentops"`) and run
`terraform init`.

### Dev overrides (local development)

To run against a locally built binary without `terraform init`, install it and
point Terraform at it with a CLI config file:

```shell
go install .   # builds terraform-provider-agentops into $GOPATH/bin
```

```hcl
# ~/.terraformrc
provider_installation {
  dev_overrides {
    "registry.terraform.io/komodorio/agentops" = "/Users/you/go/bin"
  }
  direct {}
}
```

`dev_overrides` is for iteration only — Terraform prints a warning and skips
`init`/lockfile handling while it is in effect.
