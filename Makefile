# arb-lab — build, test and structural-gate targets.
#
# Two Go modules on purpose. The root module is the engine; tools/lint is a
# nested module so that golang.org/x/tools never enters the engine's dependency
# graph (§11: platform/ is "reimplemented fresh; never a dependency"). The cost
# is that every target has to visit both modules explicitly — there is no
# go.work, so CI and a local run resolve identically.

GO      ?= go
LINTDIR := tools/lint
BIN     := $(CURDIR)/bin
LINTBIN := $(BIN)/arblab-lint

.DEFAULT_GOAL := help

.PHONY: help
help: ## list targets
	@grep -hE '^[a-z][a-z-]*:.*?## ' $(MAKEFILE_LIST) \
		| sort \
		| awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

.PHONY: ci
ci: fmt-check vet lint test ## everything CI runs

.PHONY: test
test: ## race-enabled tests, both modules
	$(GO) test -race ./...
	cd $(LINTDIR) && $(GO) test -race ./...

.PHONY: test-short
test-short: ## tests without the race detector (faster inner loop)
	$(GO) test ./...
	cd $(LINTDIR) && $(GO) test ./...

.PHONY: vet
vet: ## go vet, both modules
	$(GO) vet ./...
	cd $(LINTDIR) && $(GO) vet ./...

$(LINTBIN): $(shell find $(LINTDIR) -name '*.go' -not -path '*/testdata/*' 2>/dev/null)
	@mkdir -p $(BIN)
	cd $(LINTDIR) && $(GO) build -o $(LINTBIN) ./cmd/arblab-lint

.PHONY: lint
lint: $(LINTBIN) ## run the custom structural gates over the engine
	$(LINTBIN) ./...

.PHONY: fmt
fmt: ## format both modules
	$(GO) fmt ./...
	cd $(LINTDIR) && $(GO) fmt ./...

.PHONY: fmt-check
fmt-check: ## fail if anything is unformatted
	@out=$$(gofmt -l . 2>/dev/null); \
	if [ -n "$$out" ]; then echo "unformatted files:"; echo "$$out"; exit 1; fi

.PHONY: tidy
tidy: ## go mod tidy, both modules
	$(GO) mod tidy
	cd $(LINTDIR) && $(GO) mod tidy

.PHONY: clean
clean: ## remove build output
	rm -rf $(BIN)
