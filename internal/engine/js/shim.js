// Engine shim run by QuickJS-ng's qjs (with --std): reads one JSON request
// per line on stdin and writes one JSON response per line on stdout
// (internal/engine/protocol). Matching mirrors parity/gen/gen_node.mjs, which
// produces the Node fixtures this engine is checked against.

// Capture-group names by index, found by scanning the pattern source
// (RegExp exposes names only per match, not their group numbers).
function groupNames(p) {
  const names = [""];
  let inClass = false;
  for (let i = 0; i < p.length; i++) {
    const c = p[i];
    if (c === "\\") { i++; continue; }
    if (inClass) { if (c === "]") inClass = false; continue; }
    if (c === "[") { inClass = true; continue; }
    if (c !== "(") continue;
    if (p[i + 1] !== "?") { names.push(""); continue; }
    const m = /^\(\?<([^=!][^>]*)>/.exec(p.slice(i, i + 256));
    if (m) names.push(m[1]);
  }
  return names;
}

// One protocol.Match as JSON text. Built by hand rather than as objects for
// JSON.stringify: with hundreds of thousands of matches that halves the time.
function matchJSON(m, names) {
  const ix = m.indices;
  let groups = "";
  for (let i = 1; i < m.length; i++) {
    const g = ix[i];
    groups += (i > 1 ? "," : "") +
      '{"index":' + i +
      ',"name":' + JSON.stringify(names[i] || "") +
      ',"matched":' + (g !== undefined) +
      ',"span":[' + (g === undefined ? "-1,-1" : g[0] + "," + g[1]) + "]}";
  }
  return '{"span":[' + ix[0][0] + "," + ix[0][1] + '],"groups":[' + groups + "]}";
}

// Evaluates one request; returns the response line without its newline.
function evaluate(req) {
  const res = { id: req.id, ok: true };
  const matches = [];
  try {
    const pattern = req.pattern || "";
    const flags = req.flags || "";
    const text = req.text || "";
    new RegExp(pattern, flags); // validate with the user's own flags so errors are verbatim
    // "d" only adds match indices; add it unless the user already set it.
    const re = new RegExp(pattern, flags.includes("d") ? flags : flags + "d");
    const names = groupNames(pattern);
    const mode = req.mode || "match";
    if (mode === "replace") {
      res.output = text.replace(re, req.replacement || "");
    } else if (mode === "split") {
      const parts = text.split(re);
      res.parts = parts.map((p) => (p === undefined ? "" : p));
      res.parts_nil = parts.map((p) => p === undefined);
    } else {
      const found = flags.includes("g") ? text.matchAll(re) : [re.exec(text)].filter(Boolean);
      for (const m of found) matches.push(matchJSON(m, names));
    }
  } catch (e) {
    return JSON.stringify({ id: req.id, ok: false, error: { message: e.message, offset: -1 } });
  }
  const head = JSON.stringify(res);
  if (matches.length === 0) return head;
  return head.slice(0, -1) + ',"matches":[' + matches.join(",") + "]}";
}

const version = navigator.userAgent.replace(/^quickjs-ng\//, "");

for (;;) {
  const line = std.in.getline();
  if (line === null) break;
  if (line === "") continue;
  const req = JSON.parse(line);
  const out = req.op === "ping" ? JSON.stringify({ id: req.id, ok: true, version }) : evaluate(req);
  std.out.puts(out + "\n");
  std.out.flush();
}
