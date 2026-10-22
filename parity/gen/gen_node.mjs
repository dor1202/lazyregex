// Runs parity corpus cases through Node's real RegExp engine.
// Reads a JSON array of cases on stdin, writes a JSON array of results to stdout.
import { readFileSync } from "node:fs";

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

function groupsOf(m, names) {
  const out = [];
  for (let i = 1; i < m.length; i++) {
    const ix = m.indices[i];
    out.push({
      index: i,
      name: names[i] || "",
      matched: ix !== undefined,
      span: ix === undefined ? [-1, -1] : [ix[0], ix[1]],
    });
  }
  return out;
}

function runCase(c) {
  const res = { id: c.id, ok: true, matches: [], unit: "utf16" };
  try {
    const flags = c.flags || "";
    const glob = flags.includes("g");
    const re = new RegExp(c.pattern, flags + "d");
    const names = groupNames(c.pattern);
    const mode = c.mode || "match";
    if (mode === "replace") {
      res.output = c.text.replace(re, c.replacement || "");
    } else if (mode === "split") {
      const parts = c.text.split(re);
      res.parts = parts.map((p) => (p === undefined ? "" : p));
      res.parts_nil = parts.map((p) => p === undefined);
    } else {
      const found = glob ? [...c.text.matchAll(re)] : [re.exec(c.text)].filter(Boolean);
      for (const m of found) {
        res.matches.push({ span: [m.indices[0][0], m.indices[0][1]], groups: groupsOf(m, names) });
      }
    }
  } catch (e) {
    return { id: c.id, ok: false, error_message: e.message, matches: [], unit: "" };
  }
  return res;
}

const cases = JSON.parse(readFileSync(0, "utf8"));
process.stdout.write(JSON.stringify(cases.map(runCase)));
