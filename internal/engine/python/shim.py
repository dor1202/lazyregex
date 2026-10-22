"""Engine shim run by CPython's python.wasm: reads one JSON request per line
on stdin and writes one JSON response per line on stdout
(internal/engine/protocol). The code between the "shared" markers is a
byte-for-byte copy of parity/gen/gen_python.py's (TestShimMatchesGenerator),
so this engine evaluates exactly as the native fixtures were generated.
"""
import json
import re
import sys

# --- shared ---
_FLAGS = {
    "i": re.IGNORECASE,
    "m": re.MULTILINE,
    "s": re.DOTALL,
    "x": re.VERBOSE,
    "a": re.ASCII,
}


def _group_entries(m, rx, names):
    out = []
    for i in range(1, rx.groups + 1):
        s, e = m.span(i)
        out.append({
            "index": i,
            "name": names.get(i, ""),
            "matched": s != -1,
            "span": [s, e],
        })
    return out


def run_case(case):
    res = {"id": case["id"], "ok": True, "matches": [], "unit": "codepoints"}
    try:
        flags = 0
        glob = False
        for ch in case.get("flags", ""):
            if ch == "g":
                glob = True
            elif ch in _FLAGS:
                flags |= _FLAGS[ch]
            else:
                raise ValueError("unknown flag: " + ch)
        rx = re.compile(case["pattern"], flags)
        names = {v: k for k, v in rx.groupindex.items()}
        text = case["text"]
        mode = case.get("mode") or "match"
        if mode == "replace":
            res["output"] = rx.sub(case.get("replacement", ""), text, count=0 if glob else 1)
        elif mode == "split":
            parts = rx.split(text)
            res["parts"] = ["" if p is None else p for p in parts]
            res["parts_nil"] = [p is None for p in parts]
        else:
            if glob:
                found = list(rx.finditer(text))
            else:
                m = rx.search(text)
                found = [m] if m else []
            for m in found:
                res["matches"].append({
                    "span": list(m.span()),
                    "groups": _group_entries(m, rx, names),
                })
    except (re.error, ValueError, IndexError, OverflowError, RecursionError) as e:
        return {"id": case["id"], "ok": False, "error_message": str(e), "matches": [], "unit": ""}
    except Exception as e:  # anything else is still reported, never fatal
        return {"id": case["id"], "ok": False, "error_message": str(e), "matches": [], "unit": ""}
    return res
# --- shared ---


def _error_offset(req):
    """Position of the error in the pattern, or -1 if it is not the pattern's
    (a bad flag or replacement template)."""
    flags = 0
    for ch in req.get("flags", ""):
        if ch != "g" and ch not in _FLAGS:
            return -1
        flags |= _FLAGS.get(ch, 0)
    try:
        re.compile(req.get("pattern", ""), flags)
    except re.error as e:
        return -1 if e.pos is None else e.pos
    except Exception:
        pass
    return -1


def respond(req):
    if req.get("op") == "ping":
        return {"id": req["id"], "ok": True, "version": sys.version.split()[0]}
    case = {
        "id": req["id"],
        "pattern": req.get("pattern", ""),
        "flags": req.get("flags", ""),
        "text": req.get("text", ""),
        "mode": req.get("mode", ""),
        "replacement": req.get("replacement", ""),
    }
    res = run_case(case)
    if not res["ok"]:
        return {"id": req["id"], "ok": False,
                "error": {"message": res["error_message"], "offset": _error_offset(case)}}
    out = {"id": req["id"], "ok": True}
    for key in ("matches", "output", "parts", "parts_nil"):
        if key in res:
            out[key] = res[key]
    return out


def main():
    stdin, stdout = sys.stdin.buffer, sys.stdout.buffer
    for line in stdin:
        if not line.strip():
            continue
        out = json.dumps(respond(json.loads(line)), ensure_ascii=True)
        stdout.write(out.encode("ascii") + b"\n")
        stdout.flush()


main()
