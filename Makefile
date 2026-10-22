VERSION ?= dev
LDFLAGS := -X github.com/dor1202/lazyregex/internal/version.Version=$(VERSION)

.PHONY: build test lint wasm parity-gen

build:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o bin/lazyregex ./cmd/lazyregex

test:
	go test ./...

lint:
	golangci-lint run

# Filled in by later tasks (WASM engine builds).
wasm:
	@echo "wasm: not implemented yet"

# Regenerate parity fixtures from the real native runtime: make parity-gen FLAVOR=python|js|pcre|go
parity-gen:
	@test -n "$(FLAVOR)" || (echo "usage: make parity-gen FLAVOR=python|js|pcre|go" && exit 1)
	go run ./parity/gen/driver -flavor $(FLAVOR) -out parity/expected
