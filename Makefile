GOLANGCI_VERSION=v1.50.1
COVERAGE=coverage.out

.PHONY: test
test: ## Run unit-tests
	@go test -race -cover -count=1 -coverprofile $(COVERAGE)  ./...