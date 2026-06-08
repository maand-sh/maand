# Maand — local build and test helpers.
# SQLite requires CGO (see docs/build.md).

CGO_ENABLED ?= 1
export CGO_ENABLED

BINARY          ?= maand
# Library/cmd packages only — excludes maand/tests and maand/tests/integration.
UNIT_PKGS       := $(shell go list ./... | grep -vE '/tests(/integration)?$$')
UNIT_TIMEOUT    ?= 120s
INTEGRATION_TIMEOUT ?= 25m
GO_TEST_FLAGS   ?=

.PHONY: all build test test-unit test-tests test-integration test-all ci clean help

all: build test

help:
	@echo "Usage: make <target>"
	@echo ""
	@echo "  build             Build $(BINARY) (CGO_ENABLED=$(CGO_ENABLED))"
	@echo "  test              Run unit packages + ./tests (default)"
	@echo "  test-unit         Run library/cmd unit tests (CI; no integration)"
	@echo "  ci                Same as GitHub Actions: test-unit + build"
	@echo "  test-tests        Run ./tests package tests"
	@echo "  test-integration  Run integration tests (real workers; see assets/README.md)"
	@echo "  test-all          test + test-integration"
	@echo "  clean             Remove $(BINARY)"
	@echo ""
	@echo "Variables: BINARY, CGO_ENABLED, UNIT_TIMEOUT, INTEGRATION_TIMEOUT, GO_TEST_FLAGS"

build:
	go build -o $(BINARY) .

test: test-unit test-tests

test-unit:
	go test $(GO_TEST_FLAGS) $(UNIT_PKGS) -count=1 -timeout $(UNIT_TIMEOUT)

test-tests:
	go test $(GO_TEST_FLAGS) ./tests -count=1 -timeout $(UNIT_TIMEOUT)

test-integration:
	go test $(GO_TEST_FLAGS) -tags=integration ./tests/integration/... -count=1 -timeout $(INTEGRATION_TIMEOUT) -v

test-all: test test-integration

ci: test-unit build

clean:
	rm -f $(BINARY)
