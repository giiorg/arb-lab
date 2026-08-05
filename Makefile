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
ci: fmt-check tutorial-ka-check vet lint test ## everything CI runs

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

# -fuzz takes exactly one package AND one target, so each target gets its own
# goal rather than a loop — a loop would hide which one is running and which one
# found something. `-run '^$$'` suppresses the ordinary tests so the time budget
# goes to fuzzing; every seed corpus still runs as a table under plain `make
# test`, which is what makes these explorers rather than gates — the same
# division as test / test-properties above. A find lands in testdata/fuzz/ and
# is committed.
#
# FUZZTIME is per target, so `make fuzz` costs FUZZTIME x the number of targets.
FUZZTIME ?= 60s

.PHONY: fuzz
fuzz: fuzz-decimal fuzz-capture ## bounded fuzz run of every target (FUZZTIME=60s each)

.PHONY: fuzz-decimal
fuzz-decimal: ## bounded fuzz run of the decimal parser
	$(GO) test -run '^$$' -fuzz FuzzParseMantissa -fuzztime $(FUZZTIME) \
		./internal/platform/fixedpoint

.PHONY: fuzz-capture
fuzz-capture: ## bounded fuzz run of the capture frame decoder
	$(GO) test -run '^$$' -fuzz FuzzDecodeRecord -fuzztime $(FUZZTIME) \
		./internal/engine/capture

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

# docs/tutorial.ka.md is GENERATED from the fragments in docs/i18n/ka/tutorial/
# and never hand-edited. The generator lives under docs/i18n/tools/ — not tools/
# — so these targets are what makes it findable from the repo root; without them
# a grep of Makefile and tools/ reads as "there is no generator". The check is
# node-only on purpose: the fuller bun suite (docs/i18n/tests) does not run in
# CI, and a guard nothing runs is the same stranding one layer out.
.PHONY: tutorial-ka
tutorial-ka: ## regenerate docs/tutorial.ka.md from the Georgian fragments
	node docs/i18n/tools/assemble.mjs

.PHONY: tutorial-ka-check
tutorial-ka-check: ## fail if docs/tutorial.ka.md is stale against its fragments
	@node docs/i18n/tools/assemble.mjs --verify

.PHONY: tidy
tidy: ## go mod tidy, both modules
	$(GO) mod tidy
	cd $(LINTDIR) && $(GO) mod tidy

.PHONY: clean
clean: ## remove build output
	rm -rf $(BIN)
