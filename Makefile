# golibreofficekit — project Makefile.
# Every target is idempotent and safe to re-run.

SHELL        := /usr/bin/env bash
.SHELLFLAGS  := -eu -o pipefail -c

GO           ?= go
GOFLAGS      ?=
PKG          := ./...
COVER_OUT    := coverage.out
COVER_HTML   := coverage.html
INTEGRATION_TAG := lok_integration

.PHONY: all build test test-integration cover lint fmt fmt-check vet tidy clean

all: fmt-check vet test

build:
	$(GO) build $(GOFLAGS) $(PKG)

test:
	$(GO) test -race $(GOFLAGS) $(PKG)

test-integration:
	$(GO) test -race -tags=$(INTEGRATION_TAG) $(GOFLAGS) $(PKG)

cover:
	$(GO) test -covermode=atomic -coverprofile=$(COVER_OUT) $(PKG)
	$(GO) tool cover -func=$(COVER_OUT) | tail -n 1
	$(GO) tool cover -html=$(COVER_OUT) -o $(COVER_HTML)

vet:
	$(GO) vet $(PKG)

# Rewrite in place.
fmt:
	gofmt -w .

# Read-only guard used by CI and `make lint`. Fails if any file is
# unformatted — never rewrites.
fmt-check:
	@unformatted="$$(gofmt -l .)"; \
	if [ -n "$$unformatted" ]; then \
	  echo "gofmt needed on:"; echo "$$unformatted"; exit 1; \
	fi

lint: vet fmt-check

tidy:
	$(GO) mod tidy

clean:
	rm -f $(COVER_OUT) $(COVER_HTML)

# Coverage gate. Append packages to COVER_GATE_PKGS as they are added.
COVER_GATE_PKGS := ./internal/lokc/... ./lok/...
COVER_GATE_MIN  := 90.0

.PHONY: cover-gate
cover-gate:
	$(GO) test -race -covermode=atomic -coverprofile=$(COVER_OUT) $(COVER_GATE_PKGS)
	@total=$$( $(GO) tool cover -func=$(COVER_OUT) | awk '/^total:/ {print $$3}' | tr -d '%' ); \
	if [ -z "$$total" ]; then \
	  echo "cover-gate: no 'total:' line in $(COVER_OUT) — is the profile empty?" >&2; \
	  exit 2; \
	fi; \
	awk -v t="$$total" -v m="$(COVER_GATE_MIN)" 'BEGIN { \
	  if (t+0 < m+0) { printf "coverage %.1f%% < %.1f%% (gate)\n", t, m; exit 1 } \
	  printf "coverage %.1f%% >= %.1f%% ok\n", t, m \
	}'
