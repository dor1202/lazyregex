VERSION ?= dev
LDFLAGS := -X github.com/dor1202/lazyregex/internal/version.Version=$(VERSION)

.PHONY: build test lint release-snapshot wasm wasm-pcre wasm-js wasm-python parity-gen gifs

build:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o bin/lazyregex ./cmd/lazyregex

test:
	go test ./...

lint:
	golangci-lint run

# Local GoReleaser dry run into dist/ (nothing is published). chocolatey needs
# Windows' choco, so it is skipped here; the release workflow runs it on windows-latest.
release-snapshot:
	goreleaser release --snapshot --clean --skip=chocolatey

# WASM engine builds: Docker + pinned, checksum-verified sources (wasm/versions.env).
include wasm/versions.env
WASM_BUILD_ARGS := $(foreach v,WASI_SDK_VERSION WASI_SDK_SHA256_ARM64 WASI_SDK_SHA256_X86_64 PCRE2_VERSION PCRE2_SHA256 QUICKJS_NG_VERSION QUICKJS_NG_SHA256 CPYTHON_VERSION CPYTHON_SHA256 CPYTHON_WASI_SDK_VERSION CPYTHON_WASI_SDK_SHA256_ARM64 CPYTHON_WASI_SDK_SHA256_X86_64,--build-arg $(v)=$($(v)))

wasm: wasm-pcre wasm-js wasm-python

wasm-pcre:
	docker build -f wasm/Dockerfile --target pcre $(WASM_BUILD_ARGS) --output type=local,dest=internal/engine/pcre .

wasm-js:
	docker build -f wasm/Dockerfile --target quickjs $(WASM_BUILD_ARGS) --output type=local,dest=internal/engine/js .

wasm-python:
	docker build -f wasm/Dockerfile --target python $(WASM_BUILD_ARGS) --output type=local,dest=internal/engine/python .

# Regenerate parity fixtures from the real native runtime: make parity-gen FLAVOR=python|js|pcre|go
parity-gen:
	@test -n "$(FLAVOR)" || (echo "usage: make parity-gen FLAVOR=python|js|pcre|go" && exit 1)
	go run ./parity/gen/driver -flavor $(FLAVOR) -out parity/expected

# Regenerate the README demo GIFs (assets/*.gif) with the vhs Docker image.
gifs:
	CGO_ENABLED=0 GOOS=linux GOARCH=$$(docker version --format '{{.Server.Arch}}') go build -ldflags "-X github.com/dor1202/lazyregex/internal/version.Version=$$(git describe --tags --abbrev=0)" -o assets/tapes/lazyregex-linux ./cmd/lazyregex
	for t in demo flavors replace; do docker run --rm -v "$$PWD":/vhs ghcr.io/charmbracelet/vhs assets/tapes/$$t.tape || exit 1; done
