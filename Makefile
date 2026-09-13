.PHONY: help build test test-race vet fmt lint \
        bench bench-baseline bench-compare bench-clean \
        tools clean

BASELINE  := bench-baseline.txt
CURRENT   := bench-current.txt
COUNT     ?= 10
BENCHTIME ?= 1s
PKGS      ?= ./...

## help: show available targets
help:
	@echo "Usage: make [target]"
	@echo ""
	@sed -n 's/^## //p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/  /'

## build: compile all packages
build:
	go build $(PKGS)

## test: run tests with race detector
test:
	go test -race -shuffle=on -count=1 $(PKGS)

## vet: run go vet
vet:
	go vet $(PKGS)

## fmt: format sources
fmt:
	gofmt -s -w .
	gofumpt -w .
	goimports -w .

## lint: run golangci-lint
lint:
	golangci-lint run --timeout=5m

## bench: run all benchmarks
bench:
	go test -bench=. -benchmem -benchtime=$(BENCHTIME) $(PKGS)

## bench-baseline: record the current run as the baseline
##   Overwrites $(BASELINE).
bench-baseline:
	go test -bench=. -benchmem -benchtime=$(BENCHTIME) -count=$(COUNT) $(PKGS) > $(BASELINE)
	@echo "baseline written to $(BASELINE)"

## bench-compare: run benchmarks and compare against the baseline
##   Requires: make tools
bench-compare:
	go test -bench=. -benchmem -benchtime=$(BENCHTIME) -count=$(COUNT) $(PKGS) > $(CURRENT)
	benchstat $(BASELINE) $(CURRENT)

## bench-clean: remove benchmark artifacts
bench-clean:
	rm -f $(BASELINE) $(CURRENT)

## tools: install development tools (benchstat, gofumpt, goimports, golangci-lint)
tools:
	go install golang.org/x/perf/cmd/benchstat@latest
	go install mvdan.cc/gofumpt@latest
	go install golang.org/x/tools/cmd/goimports@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

## clean: remove build and benchmark artifacts
clean: bench-clean
	go clean -testcache
