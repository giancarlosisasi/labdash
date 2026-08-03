# labdash — the entry points a contributor and CI both use.
#
# The rule that matters here: `test-update` rewrites golden files and `test`
# never does. CI runs `test`, so a stale golden file fails the build instead of
# quietly rewriting itself. See website/docs/en/about/contributing.mdx.

GO      ?= go
BINARY  ?= labdash
PKG     := ./...

.DEFAULT_GOAL := help

## help: list the targets
help:
	@echo "labdash targets:"
	@sed -n 's/^## //p' $(MAKEFILE_LIST) | awk -F: '{printf "  %-14s %s\n", $$1, $$2}'

## build: compile the binary
build:
	$(GO) build -o $(BINARY) ./cmd/labdash

## test: run every test. Never passes -update.
test:
	$(GO) test $(PKG)

## test-race: the same suite under the race detector
test-race:
	$(GO) test -race $(PKG)

## test-update: rewrite every golden file from the current output
test-update:
	$(GO) test $(PKG) -update

## bench: run the latency budgets
bench:
	$(GO) test $(PKG) -run '^$$' -bench . -benchmem

## preview: render the design system in this terminal
preview:
	$(GO) run ./cmd/labdash theme preview

## vet: go vet over everything
vet:
	$(GO) vet $(PKG)

## tidy: sync go.mod and go.sum
tidy:
	$(GO) mod tidy

## ci: exactly what the pipeline runs
ci: vet test test-race bench

.PHONY: help build test test-race test-update bench preview vet tidy ci
