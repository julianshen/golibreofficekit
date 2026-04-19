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
