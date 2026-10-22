VERSION ?= dev
LDFLAGS := -X github.com/dor1202/lazyregex/internal/version.Version=$(VERSION)

.PHONY: build test lint wasm wasm-pcre parity-gen

build:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o bin/lazyregex ./cmd/lazyregex

test:
	go test ./...

lint:
	golangci-lint run

# WASM engine builds: Docker + pinned, checksum-verified sources (wasm/versions.env).
include wasm/versions.env
WASM_BUILD_ARGS := $(foreach v,WASI_SDK_VERSION WASI_SDK_SHA256_ARM64 WASI_SDK_SHA256_X86_64 PCRE2_VERSION PCRE2_SHA256,--build-arg $(v)=$($(v)))

wasm: wasm-pcre

wasm-pcre:
	docker build -f wasm/Dockerfile --target pcre $(WASM_BUILD_ARGS) --output type=local,dest=internal/engine/pcre .

# Regenerate parity fixtures from the real native runtime: make parity-gen FLAVOR=python|js|pcre|go
parity-gen:
	@test -n "$(FLAVOR)" || (echo "usage: make parity-gen FLAVOR=python|js|pcre|go" && exit 1)
	go run ./parity/gen/driver -flavor $(FLAVOR) -out parity/expected
