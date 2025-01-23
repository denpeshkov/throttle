.PHONY: all
all: tidy lint test

MODULE_DIRS = . ./internal/throttletest

.PHONY: help
help: ## Display this help screen
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.PHONY: tidy
tidy: ## Tidy
	@$(foreach mod,$(MODULE_DIRS), \
		(cd $(mod) && \
		go mod tidy -v && \
		go mod verify && \
		go fmt ./... &&\
		go vet ./... && \
		staticcheck ./...) &&) true

.PHONY: lint
lint: ## Lint
	@$(foreach mod,$(MODULE_DIRS), \
		(cd $(mod) && golangci-lint run --path-prefix $(mod) ./...) &&) true


.PHONY: test
test: ## Test
	@$(foreach mod,$(MODULE_DIRS), \
		(cd $(mod) && go test -race -buildvcs -count=1 ./...) &&) true
