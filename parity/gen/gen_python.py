#!/usr/bin/env python3
"""Runs parity corpus cases through Python's real `re` module.

Reads a JSON array of cases on stdin, writes a JSON array of results to stdout.
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


def main():
    cases = json.load(sys.stdin)
    json.dump([run_case(c) for c in cases], sys.stdout, ensure_ascii=True)


if __name__ == "__main__":
    main()
