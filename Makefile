VERSION ?= dev
LDFLAGS := -X github.com/dor1202/lazyregex/internal/version.Version=$(VERSION)

.PHONY: build test lint wasm

build:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o bin/lazyregex ./cmd/lazyregex

test:
	go test ./...

lint:
	golangci-lint run

# Filled in by later tasks (WASM engine builds).
wasm:
	@echo "wasm: not implemented yet"
