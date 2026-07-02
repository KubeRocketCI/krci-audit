CURRENT_DIR=$(shell pwd)
HOST_OS?=$(shell go env GOOS)
HOST_ARCH?=$(shell go env GOARCH)
DIST_DIR=${CURRENT_DIR}/dist

LOCALBIN ?= ${CURRENT_DIR}/bin
GOLANGCI_LINT_VERSION ?= v2.1.6
HELMDOCS_VERSION ?= v1.14.2
# When a read API with internal/api/oapi.yaml is added, wire OAPICODEGEN_VERSION + a
# `generate` target (oapi-codegen passes for server + models). Omitted now: no OpenAPI spec yet.

$(LOCALBIN):
	mkdir -p $(LOCALBIN)

.PHONY: build
build: ## build the migration runner binary
	CGO_ENABLED=0 GOOS=${HOST_OS} GOARCH=${HOST_ARCH} go build -v -o ${DIST_DIR}/krci-audit-migrate-${HOST_ARCH} ./cmd/krci-audit-migrate

.PHONY: test
test: ## run unit + integration tests (integration needs Docker; helm render needs helm)
	go test ./... -coverprofile=coverage.out

.PHONY: test-unit
test-unit: ## run only the fast unit tests (no Docker)
	go test ./internal/config/... ./internal/dsn/... ./internal/models/... ./pkg/identity/... -coverprofile=coverage.out

.PHONY: lint
lint: golangci-lint ## run go lint
	${LOCALBIN}/golangci-lint run -v -c .golangci.yaml ./...

.PHONY: lint-fix
lint-fix: golangci-lint ## run go lint with --fix
	${LOCALBIN}/golangci-lint run -v -c .golangci.yaml ./... --fix

.PHONY: helm-lint
helm-lint: ## lint the Helm chart
	helm lint deploy-templates

.PHONY: helm-template
helm-template: ## render the Helm chart with defaults
	helm template krci-audit deploy-templates --namespace krci-audit

.PHONY: helm-docs
helm-docs: helmdocs ## generate deploy-templates/README.md from values
	${LOCALBIN}/helm-docs

.PHONY: helmdocs
helmdocs: $(LOCALBIN)
	@test -x $(LOCALBIN)/helm-docs || \
		GOBIN=$(LOCALBIN) go install github.com/norwoodj/helm-docs/cmd/helm-docs@$(HELMDOCS_VERSION)

.PHONY: golangci-lint
golangci-lint: $(LOCALBIN)
	@test -x $(LOCALBIN)/golangci-lint || \
		GOBIN=$(LOCALBIN) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

.PHONY: help
help: ## show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-16s\033[0m %s\n", $$1, $$2}'
