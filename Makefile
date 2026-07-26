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

# rapid's -rapid.seed defaults to 0, which rapid reads as "choose a random one"
# (baseSeed() in data.go hashes the clock when the flag is zero). That makes the
# property tests non-deterministic out of the box, which is the wrong default
# for a gate: a green build would not be reproducible and a red one might not
# replay. So the everyday and CI runs pin the seed, and one target deliberately
# does not — see test-properties.
#
# RAPID_SEED and RAPID_CHECKS are read by rapid itself as environment variables,
# equivalent to the -rapid.seed and -rapid.checks flags.
RAPID_SEED           ?= 20260726
RAPID_NIGHTLY_CHECKS ?= 20000

.DEFAULT_GOAL := help

.PHONY: help
help: ## list targets
	@grep -hE '^[a-z][a-z-]*:.*?## ' $(MAKEFILE_LIST) \
		| sort \
		| awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2}'

.PHONY: ci
ci: fmt-check vet lint test ## everything CI runs

.PHONY: test
test: ## race-enabled tests, both modules (pinned rapid seed)
	RAPID_SEED=$(RAPID_SEED) $(GO) test -race ./...
	cd $(LINTDIR) && $(GO) test -race ./...

.PHONY: test-short
test-short: ## tests without the race detector (faster inner loop)
	RAPID_SEED=$(RAPID_SEED) $(GO) test ./...
	cd $(LINTDIR) && $(GO) test ./...

.PHONY: test-properties
test-properties: ## deep property run on a RANDOM seed — the nightly explorer
	@echo "rapid seed: random (this run is not reproducible; note the seed it prints on failure)"
	RAPID_CHECKS=$(RAPID_NIGHTLY_CHECKS) $(GO) test -count=1 -run 'TestProperty' ./...

.PHONY: test-properties-deep
test-properties-deep: ## deep property run on the pinned seed (reproducible)
	RAPID_CHECKS=$(RAPID_NIGHTLY_CHECKS) RAPID_SEED=$(RAPID_SEED) \
		$(GO) test -count=1 -run 'TestProperty' ./...

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
