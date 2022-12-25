.DEFAULT_GOAL := help

.PHONY: setup
setup: ## Setup tools
	go install github.com/golang/mock/mockgen

.PHONY: test
test: ## Run tests
	go install gotest.tools/gotestsum@latest
	gotestsum -- -race -coverprofile=coverage.out ./...

.PHONY: cover
cover: test ## Run tests with showing coverage
	go tool cover -func=coverage.out
	go tool cover -html=coverage.out

.PHONY: lint
lint: ## Run golangci-lint
	@golangci-lint run

.PHONY: generate
generate: ## Run go generate
	go generate ./...

.PHONY: fmt
fmt: ## Format code
	goimports -w .

.PHONY: help
help: ## Display this help screen
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
