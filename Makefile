GOLANGCI_LINT = go tool -modfile tools/go.mod github.com/golangci/golangci-lint/v2/cmd/golangci-lint
LEFTHOOK = go tool -modfile tools/go.mod github.com/evilmartians/lefthook

.PHONY: lint
lint: golangci-lint

.PHONY: lint-fix
lint-fix: golangci-lint-fix

.PHONY: golangci-lint
golangci-lint: ## Run golangci-lint over the codebase.
	${GOLANGCI_LINT} run ./... --timeout 5m -v ${GOLANGCI_LINT_EXTRA_ARGS}

.PHONY: golangci-lint-fix
golangci-lint-fix: GOLANGCI_LINT_EXTRA_ARGS := --fix
golangci-lint-fix: golangci-lint ## Run golangci-lint over the codebase and run auto-fixers if supported by the linter

.PHONY: fmt
fmt: ## Format code with golangci-lint
	${GOLANGCI_LINT} fmt ./...

.PHONY: fmt-diff
fmt-diff: ## Show code formatting differences
	${GOLANGCI_LINT} fmt --diff ./...

# Install lefthook
.PHONY: install-lefthook
install-lefthook:
	${LEFTHOOK} install

# Manual lefthook run
.PHONY: run-lefthook
run-lefthook:
	${LEFTHOOK} run pre-commit

# Test targets
.PHONY: test
test: ## Run all tests
	go test ./... -shuffle on -v -race

.PHONY: test-coverage
test-coverage: ## Run tests with coverage report
	go test -race -coverprofile=coverage.out -covermode=atomic -coverpkg=./... ./...

.PHONY: coverage-html
coverage-html: test-coverage ## Generate HTML coverage report
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Build targets
.PHONY: build
build: ## Build wtgc binary
	go build -o bin/wtgc ./cmd/wtgc

.PHONY: install
install: ## Install wtgc binary
	go install ./cmd/wtgc

.PHONY: help
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)
