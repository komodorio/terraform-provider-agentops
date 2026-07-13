default: fmt lint install generate

build:
	go build -v ./...

install: build
	go install -v ./...

lint:
	golangci-lint run

# Refresh the vendored OpenAPI spec from the monorepo. Override the source with
# one of:
#   make sync-spec MONOREPO=../agentops            # copy from a local checkout
#   make sync-spec ENDPOINT=https://staging.host   # curl the live /api/openapi.json
MONOREPO ?= ../agentops
SPEC_PATH ?= packages/generated/openapi/agentops-controlplane.openapi.json
sync-spec:
	@mkdir -p api
ifdef ENDPOINT
	curl -fsSL "$(ENDPOINT)/api/openapi.json" -o api/openapi.json
else
	cp "$(MONOREPO)/$(SPEC_PATH)" api/openapi.json
endif
	@echo "Wrote api/openapi.json"

# Generate the typed client from the vendored spec into internal/client/gen/.
# Re-run on every API change (after `make sync-spec`).
#
# FastAPI emits OpenAPI 3.1, which oapi-codegen v2 cannot yet consume, so we
# down-convert to a 3.0 subset (api/openapi.gen.json, gitignored) first. The
# vendored api/openapi.json stays a faithful 3.1 mirror for clean refresh diffs.
generate:
	go run ./internal/specdowngrade api/openapi.json api/openapi.gen.json
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen -config oapi-codegen.yaml api/openapi.gen.json

# Generate provider documentation (tfplugindocs), copyright headers, and format
# the Terraform examples.
docs:
	cd tools; go generate ./...

fmt:
	gofmt -s -w -e .

test:
	go test -v -cover -timeout=120s -parallel=10 ./...

testacc:
	TF_ACC=1 go test -v -cover -timeout 120m ./...

.PHONY: fmt lint test testacc build install generate docs sync-spec
