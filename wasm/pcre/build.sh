#!/bin/sh
# Builds libpcre2-8.a (8-bit, Unicode, no JIT) with wasi-sdk and links the
# shim into a WASI command module, written gzip'd to $3/pcre.wasm.gz.
#   build.sh <pcre2 source dir> <shim dir> <out dir>
set -eu
PCRE2=$1
SHIM=$2
OUT=$3
CC="$WASI_SDK/bin/clang --target=wasm32-wasip1 --sysroot=$WASI_SDK/share/wasi-sysroot"
AR="$WASI_SDK/bin/llvm-ar"
CFLAGS="-O2 -Wall -DNDEBUG -ffile-prefix-map=/src=."
B=/tmp/pcre2-build
mkdir -p "$B" "$OUT"

# Same configuration as PHP's bundled PCRE2 (ext/pcre/pcre2lib/config.h);
# config.h.generic's defaults already match it (LINK_SIZE 2, newline LF,
# MAX_NAME_SIZE 128, PARENS_NEST_LIMIT 250, ...).
cp "$PCRE2/src/config.h.generic" "$B/config.h"
cp "$PCRE2/src/pcre2.h.generic" "$B/pcre2.h"
cp "$PCRE2/src/pcre2_chartables.c.dist" "$B/pcre2_chartables.c"
PCRE_DEFS="-DHAVE_CONFIG_H -DPCRE2_CODE_UNIT_WIDTH=8 -DPCRE2_STATIC -DSUPPORT_UNICODE -DHAVE_STDINT_H -DHAVE_INTTYPES_H -DHAVE_MEMMOVE"

# Library sources (PCRE2_SOURCES in PCRE2's CMakeLists.txt). Releases after
# 10.44 split the compiler and matcher into extra files; build them if present.
SRCS="auto_possess chkdint compile config context convert dfa_match error
extuni find_bracket jit_compile maketables match match_data newline ord2utf
pattern_info script_run serialize string_utils study substitute substring
tables ucd valid_utf xclass"
for s in compile_cgroup compile_class match_next; do
	if [ -f "$PCRE2/src/pcre2_$s.c" ]; then SRCS="$SRCS $s"; fi
done
OBJS=""
for s in $SRCS; do
	$CC $CFLAGS $PCRE_DEFS -I"$B" -I"$PCRE2/src" -c "$PCRE2/src/pcre2_$s.c" -o "$B/pcre2_$s.o"
	OBJS="$OBJS $B/pcre2_$s.o"
done
$CC $CFLAGS $PCRE_DEFS -I"$B" -I"$PCRE2/src" -c "$B/pcre2_chartables.c" -o "$B/pcre2_chartables.o"
OBJS="$OBJS $B/pcre2_chartables.o"
rm -f "$B/libpcre2-8.a"
$AR rcsD "$B/libpcre2-8.a" $OBJS

$CC $CFLAGS -DPCRE2_CODE_UNIT_WIDTH=8 -DPCRE2_STATIC -I"$B" -I"$SHIM" \
	"$SHIM/shim.c" "$SHIM/json.c" "$B/libpcre2-8.a" \
	-Wl,-z,stack-size=1048576 -Wl,--strip-all \
	-o "$B/pcre.wasm"
gzip -9 -n -c "$B/pcre.wasm" > "$OUT/pcre.wasm.gz"
ls -l "$B/pcre.wasm" "$OUT/pcre.wasm.gz"
