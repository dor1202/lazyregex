# Builds the regex engines as WASI command modules. Driven by the Makefile
# (make wasm-pcre, make wasm-js), which passes every pinned version from
# wasm/versions.env as a build argument and copies the final stage's files
# into the repo.

# debian:bookworm-slim (multi-arch index digest)
FROM debian@sha256:7ff6ba31527b9e336162fa061746c366c4b2032f4d2cff2078bea43f78efe3f8 AS wasi-sdk
ARG TARGETARCH
ARG WASI_SDK_VERSION
ARG WASI_SDK_SHA256_ARM64
ARG WASI_SDK_SHA256_X86_64
RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates curl gzip make \
 && rm -rf /var/lib/apt/lists/*
RUN set -eu; \
    case "$TARGETARCH" in \
      arm64) arch=arm64; sum="$WASI_SDK_SHA256_ARM64" ;; \
      amd64) arch=x86_64; sum="$WASI_SDK_SHA256_X86_64" ;; \
      *) echo "unsupported build arch $TARGETARCH" >&2; exit 1 ;; \
    esac; \
    major="${WASI_SDK_VERSION%%.*}"; \
    curl -fsSLo /tmp/wasi-sdk.tar.gz \
      "https://github.com/WebAssembly/wasi-sdk/releases/download/wasi-sdk-${major}/wasi-sdk-${WASI_SDK_VERSION}-${arch}-linux.tar.gz"; \
    echo "${sum}  /tmp/wasi-sdk.tar.gz" | sha256sum -c -; \
    mkdir /opt/wasi-sdk; \
    tar xzf /tmp/wasi-sdk.tar.gz --strip-components=1 -C /opt/wasi-sdk; \
    rm /tmp/wasi-sdk.tar.gz
ENV WASI_SDK=/opt/wasi-sdk

# ---- PCRE2 (PHP preg_* semantics) ----
FROM wasi-sdk AS pcre-build
ARG PCRE2_VERSION
ARG PCRE2_SHA256
RUN set -eu; \
    curl -fsSLo /tmp/pcre2.tar.gz \
      "https://github.com/PCRE2Project/pcre2/releases/download/pcre2-${PCRE2_VERSION}/pcre2-${PCRE2_VERSION}.tar.gz"; \
    echo "${PCRE2_SHA256}  /tmp/pcre2.tar.gz" | sha256sum -c -; \
    mkdir -p /src; \
    tar xzf /tmp/pcre2.tar.gz -C /src; \
    mv "/src/pcre2-${PCRE2_VERSION}" /src/pcre2; \
    rm /tmp/pcre2.tar.gz
COPY wasm/pcre/ /src/shim/
RUN sh /src/shim/build.sh /src/pcre2 /src/shim /out

FROM scratch AS pcre
COPY --from=pcre-build /out/pcre.wasm.gz /

# ---- QuickJS-ng (ECMAScript RegExp) ----
FROM wasi-sdk AS quickjs-build
ARG QUICKJS_NG_VERSION
ARG QUICKJS_NG_SHA256
RUN apt-get update \
 && apt-get install -y --no-install-recommends cmake=3.25.1-1 \
 && rm -rf /var/lib/apt/lists/*
RUN set -eu; \
    curl -fsSLo /tmp/quickjs.tar.gz \
      "https://github.com/quickjs-ng/quickjs/archive/refs/tags/v${QUICKJS_NG_VERSION}.tar.gz"; \
    echo "${QUICKJS_NG_SHA256}  /tmp/quickjs.tar.gz" | sha256sum -c -; \
    mkdir -p /src; \
    tar xzf /tmp/quickjs.tar.gz -C /src; \
    mv "/src/quickjs-${QUICKJS_NG_VERSION}" /src/quickjs; \
    rm /tmp/quickjs.tar.gz
COPY wasm/quickjs/ /src/build/
RUN sh /src/build/build.sh /src/quickjs /out

FROM scratch AS quickjs
COPY --from=quickjs-build /out/qjs.wasm.gz /
