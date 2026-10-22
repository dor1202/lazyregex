#!/bin/sh
# Builds CPython as a WASI command module with its own WASI tooling
# (Tools/wasm/wasi), written stripped and gzip'd to $3/python.wasm.gz, plus
# $3/stdlib.zip: the standard-library modules listed in $2, byte-compiled and
# stored uncompressed (zipimport reads stored entries without zlib, which the
# WASI build lacks).
#   build.sh <cpython source dir> <stdlib allowlist> <out dir>
set -eu
SRC=$1
ALLOW=$2
OUT=$3
mkdir -p "$OUT"
cd "$SRC"
export SOURCE_DATE_EPOCH=0

# Native "build Python" (runs the cross build's code generators), then the
# WASI host build. The build image has no WASI runtime, so the host runner
# that Tools/wasm/wasi writes into python.sh is a no-op; the module is
# exercised under wazero by internal/engine/python's tests.
python3 Tools/wasm/wasi build-python --quiet --logdir /tmp
# Extension modules the shim can never reach are left out (smaller module,
# faster wazero compile). _sre, unicodedata (\N{...} escapes) and _json stay.
python3 Tools/wasm/wasi configure-host --quiet --logdir /tmp \
	--wasi-sdk "$WASI_SDK" --host-runner true -- \
	--disable-test-modules \
	CFLAGS="-ffile-prefix-map=$SRC=." \
	$(for m in _asyncio _bisect _csv _heapq _lsprof _pickle _queue _random \
		select _struct _zoneinfo _statistics cmath _datetime _socket pyexpat \
		_elementtree _codecs_cn _codecs_hk _codecs_iso2022 _codecs_jp \
		_codecs_kr _codecs_tw _multibytecodec _md5 _sha1 _sha2 _sha3 _blake2 \
		_hmac _decimal binascii array; do echo "py_cv_module_$m=n/a"; done)
python3 Tools/wasm/wasi make-host --quiet --logdir /tmp
HOST=cross-build/wasm32-wasip1
"$WASI_SDK/bin/llvm-strip" -o /tmp/python.wasm "$HOST/python.wasm"
gzip -9 -n -c /tmp/python.wasm > "$OUT/python.wasm.gz"

# stdlib.zip, compiled by the build Python (same version, same bytecode).
# Sourceless unchecked-hash .pyc files and fixed timestamps keep the zip
# reproducible.
BUILD_PYTHON=$(ls cross-build/*-linux-gnu/python)
"$BUILD_PYTHON" -I - "$SRC/Lib" "$ALLOW" "$OUT/stdlib.zip" <<'EOF'
import importlib.util, marshal, pathlib, sys, zipfile

lib, allow, out = pathlib.Path(sys.argv[1]), sys.argv[2], sys.argv[3]
files = set()
for line in open(allow, encoding="utf-8"):
    entry = line.split("#", 1)[0].strip()
    if not entry:
        continue
    path = lib / entry
    if entry.endswith("/"):
        found = sorted(path.rglob("*.py"))
    else:
        found = [path] if path.is_file() else []
    if not found:
        sys.exit(f"stdlib allowlist: {entry} matches nothing")
    files.update(p.relative_to(lib) for p in found)

with zipfile.ZipFile(out, "w", zipfile.ZIP_STORED) as zf:
    for rel in sorted(files, key=str):
        source = (lib / rel).read_bytes()
        code = compile(source, str(rel), "exec", dont_inherit=True)
        # PEP 552 unchecked-hash pyc: flags=0b01 (hash-based, unchecked).
        pyc = bytearray(importlib.util.MAGIC_NUMBER)
        pyc += (1).to_bytes(4, "little")
        pyc += importlib.util.source_hash(source)
        pyc += marshal.dumps(code)
        info = zipfile.ZipInfo(str(rel.with_suffix(".pyc")), (1980, 1, 1, 0, 0, 0))
        info.external_attr = 0o644 << 16
        zf.writestr(info, bytes(pyc), zipfile.ZIP_STORED)
print(f"stdlib.zip: {len(files)} modules")
EOF
ls -l /tmp/python.wasm "$OUT/python.wasm.gz" "$OUT/stdlib.zip"
