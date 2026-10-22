#!/bin/sh
# Builds QuickJS-ng's qjs command-line runner as a WASI command module with
# wasi-sdk (the project's own WASI CMake configuration, as used for its
# qjs-wasi.wasm release asset), written gzip'd to $2/qjs.wasm.gz.
#   build.sh <quickjs-ng source dir> <out dir>
set -eu
SRC=$1
OUT=$2
B=/tmp/quickjs-build
mkdir -p "$OUT"

cmake -S "$SRC" -B "$B" \
	-DCMAKE_TOOLCHAIN_FILE="$WASI_SDK/share/cmake/wasi-sdk-p1.cmake" \
	-DCMAKE_BUILD_TYPE=Release \
	-DCMAKE_C_FLAGS="-ffile-prefix-map=/src=." \
	-DCMAKE_EXE_LINKER_FLAGS="-Wl,--strip-all" \
	-DBUILD_SHARED_LIBS=OFF
cmake --build "$B" --target qjs_exe -j"$(nproc)"
gzip -9 -n -c "$B/qjs" > "$OUT/qjs.wasm.gz"
ls -l "$B/qjs" "$OUT/qjs.wasm.gz"
