# lazyregex v1.0: Go rewrite — design

Date: 2026-10-20
Status: reviewed, awaiting approval
Mockups: https://claude.ai/artifact/XY77PVRXBRS2jyg6XW2prp (boards 6–9 are the chosen
layout; boards 1–5 are the rejected k9s-style alternative, kept for reference)

## Intent

lazyregex is a terminal regex tester: regex101, but local, offline, in the
terminal. Today it is a Python/Textual app. It looks rough, it only speaks
Python's `re`, and shipping it through package managers is painful (the
Homebrew formula has to vendor a Python virtualenv and depends on
`python@3.13`).

v1.0 rewrites it in Go, in place in `dor1202/lazyregex`, as a single
self-contained binary with a lazygit-style interface.

The one requirement that drives everything else: **a developer must be able
to trust that what lazyregex shows is exactly what their language does.** A
Python developer testing a pattern must see exactly what Python's `re` would
return — same matches, same groups, same spans, same errors. "Close enough"
is a failure.

## Goals

1. Four flavors, each run by that language's real regex engine, bundled
   into the binary:
   - **Python** — CPython's `re`
   - **JavaScript** — ECMAScript `RegExp`
   - **PCRE** — PCRE2
   - **Go** — Go's `regexp` (RE2 syntax)
2. Good explanations of a pattern, regex101-quality: a tree breaking the
   pattern into tokens, each with a plain-English line, aware of the
   selected flavor's syntax.
3. A lazygit-style TUI that is fully usable from the keyboard alone, with
   optional mouse support.
4. Installs with one command from package managers, with nothing else
   needed on the machine — no Python, no Node, no shared libraries.
5. Fast: instant startup feel, results update live as you type.

## Non-goals (v1)

- Homebrew **core** submission (tap only for now; core can come later).
- Updating or yanking the old Python release. v0.x stays reachable via its
  git tags; it was never published to PyPI.
- regex101's step-through debugger and unit-test features.
- Sharing patterns online / any network feature.
- Flavors beyond the four (Java, .NET, Rust, …). The architecture should
  not make adding one hard, but none ship in v1.

## Anti-goals (failure even if it "works")

- Any flavor whose results come from an emulation or translation layer
  rather than that language's real engine. E.g. running JS patterns through
  a Go port of a different engine, or Python patterns through PCRE.
- Using whatever `python3` / `node` happens to be installed on the user's
  machine. The decision is explicit: **bundle only, assume nothing about
  the user's machine.**
- A feature reachable only by mouse.
- Silently rewriting the user's pattern (e.g. when switching flavor). The
  app may *suggest* a rewrite; the user applies it explicitly.
- Network access at runtime, including telemetry or update checks.

## Constraints

- Language: Go.
- Output: a single static binary per platform. Target `CGO_ENABLED=0` so
  cross-compilation stays trivial in GoReleaser (my call; see Engines).
- Platforms: macOS (amd64, arm64), Linux (amd64, arm64), Windows (amd64,
  arm64 optional).
- Must work fully offline.
- Released from the existing repo `dor1202/lazyregex`, replacing the
  Python code on a branch merged to `main`. First Go release is **v1.0.0**.
- MIT licence stays. Bundled engines' licences (PSF for CPython, BSD for
  PCRE2, MIT for QuickJS) must be included in the release archives and
  shown under `lazyregex --licenses` (my call).

## Engines (how exactness is achieved)

Decided with the user: real engines, bundled. The following implementation
is **my call** and can be changed by the builder as long as the constraints
above and the anti-goals hold.

| Flavor | Engine | How it is bundled |
|---|---|---|
| Go | Go stdlib `regexp` | native, nothing to bundle |
| PCRE | PCRE2 (pinned release) | compiled to WebAssembly, run in-process with `wazero` (pure-Go WASM runtime) |
| JavaScript | QuickJS (or QuickJS-ng) `libregexp` via the JS `RegExp` API | compiled to WASM, run with `wazero` |
| Python | CPython (pinned release, WASI build) with a trimmed stdlib containing `re` and its dependencies | WASI module run with `wazero` |

Why WASM + wazero: it keeps the binary pure Go (no cgo, no shared
libraries), so GoReleaser can cross-compile everything from one Linux
runner, and each engine is the real C implementation, not a port.
Alternatives the builder may use if they hold the same guarantees: a
ccgo-transpiled pure-Go PCRE2 (e.g. `go.elara.ws/pcre`). Not acceptable:
`dlclark/regexp2` or `goja`'s regexp for JS/PCRE/Python (they are
re-implementations, see anti-goals).

**JavaScript exactness (decided by the user):** JS runs on QuickJS, not
V8. JS is "ECMAScript-exact", not "Node-exact". Matches agree with V8 in
practice; known divergences (error message wording, Unicode property data
version, `v`-flag edge cases) are documented in the README and in a
parity-exceptions list in the test suite. A JS parity mismatch against Node
fails CI unless it is on that list. Errors shown are QuickJS's own text.

Engine requirements:

- **One interface** in Go, roughly: `Compile(pattern, flags) → (handle | error)`,
  `FindAll(text) → matches with group spans and names`, `Replace(text, repl)`,
  `Split(text)`. Each engine adapter implements it by calling the real
  engine's own API (`re.finditer`, `String.prototype.matchAll`,
  `pcre2_match` loop, `regexp.FindAllStringSubmatchIndex`, and their
  replace/split equivalents).
- **Errors are the engine's own message, unedited** (board 8 shows Go's
  real `error parsing regexp: …`). The app may add a "why / try" hint
  underneath, clearly separated.
- **Spans: show what the language itself reports.** Python reports code
  points, JS UTF-16 code units, PCRE and Go bytes. The Matches panel shows
  each flavor's native numbers (that is what `m.start()` / `m.index`
  returns in the user's code); highlighting maps them to screen cells
  internally. (My call.)
- **Versions are pinned** and shown in the Flavor panel and `--version`,
  e.g. "Python 3.13.x re". Which exact versions to pin is left to the
  builder (latest stable at build time).
- **Lazy start:** an engine is only loaded when its flavor is first used,
  and compiled WASM is cached (wazero compilation cache in the user cache
  dir) so later starts are fast. Startup with the default flavor should
  feel instant (target < 150 ms to first paint; my number).
- **Runaway patterns:** every evaluation runs with a timeout (default 2 s,
  configurable) and is cancelled on the next keystroke. A timeout shows
  "timed out — possible catastrophic backtracking", never a hang. wazero
  supports context cancellation for this.
- **Live updates:** re-evaluate on each edit with a short debounce
  (~50–100 ms; builder's call).
- **Default flavor and startup (my call):** default flavor is Python
  (configurable; the existing app is Python-only). The UI paints
  immediately; the selected engine loads in the background with a short
  "loading python engine…" state in [5]/[6]. The other three engines warm
  up in the background after first paint.
- **Cross-flavor checks (my call):** the ✗ markers in [1] are computed for
  engines that are already loaded (all of them, once warm-up finishes),
  debounced more slowly than the main evaluation (~300 ms). The Compare tab
  runs all four engines only while it is visible.
- **After a timeout (my call):** if cancelling means discarding a WASM
  instance, the adapter re-instantiates it; a brief "restarting engine"
  state is acceptable. Keeping one spare warm instance per engine is the
  builder's call.
- **Text input (my call):** the test string is text. Invalid UTF-8 (e.g.
  piped binary logs) is decoded with U+FFFD replacement and a visible
  warning, so all four engines see the same text. Above ~1 MB of test
  text, live updates pause and evaluation runs on `enter`, with a notice.

Per-flavor flags (shown in the Flags panel; only the selected flavor's
flags appear):

- Python: `i m s x a` (`u` is implicit for str), plus `g` meaning
  `finditer` vs single `search`.
- JavaScript: `d g i m s u v y` — passed straight to `RegExp`.
- PCRE: `i m s x n U u` and others PCRE2 exposes that the builder judges
  useful, plus `g`. **PCRE stands in for PHP's `preg_*` (my call):** it is
  configured the way PHP configures PCRE2 (PHP's `/u` → UTF mode, PHP's
  newline/compile defaults), `u` is **on by default** since the text is
  UTF-8, the Code tab emits PHP, and CI parity for PCRE is checked against
  PHP's `preg_match_all` / `preg_replace` / `preg_split`.
- Go: `i m s U` (as inline `(?flags)`), plus `g` meaning `FindAll` vs `Find`.

## Explanations

The Explanation tab shows a tree of the pattern's tokens with a plain-English
line per token (boards 6 and 9), flavor-aware (e.g. `(?P<name>…)` in
Python, `(?<name>…)` in JS, `\K` / recursion / possessive in PCRE).

- Engines don't produce explanations, so lazyregex needs its own parser per
  flavor (or one parser with flavor-specific dialects) that builds a token
  tree. How is left to the builder.
- **The engine is the authority.** If the real engine rejects a pattern,
  the error is shown even if the explainer could parse it. If the engine
  accepts a pattern the explainer can't parse, show "can't explain this
  part" for that token rather than guessing.
- Selecting a row in the explanation underlines the matching part of the
  pattern (and clicking a pattern token selects its row).
- Quality bar: comparable to regex101's explanation panel for common
  constructs (anchors, classes, escapes, quantifiers incl. lazy/possessive,
  groups of every kind, lookarounds, backreferences, alternation, flags,
  Unicode properties).

## The interface (decided: lazygit style)

See mockup boards 6–9. A left column of stacked list panels and a right
area that shows the work; rounded borders; the focused panel's border is
green; panel titles carry `[n]` numbers and tabs.

| Panel | Contents |
|---|---|
| [1] Flavor | python / js / pcre / go with engine names; ✗ marker when the current pattern doesn't compile in that flavor |
| [2] Flags | the selected flavor's flags as checkboxes |
| [3] Matches ─ Groups | each match, expandable to its groups (name, span, value); selected match is outlined in [6] |
| [4] Library | saved patterns (name, flavor) |
| [5] Pattern: Match ─ Replace ─ Split | the pattern editor; Replace adds a replacement line in the flavor's own syntax (`\g<name>` for Python, `$<name>` for JS, …); Split runs the flavor's split |
| [6] Test String | multi-line editor with match highlighting, one colour per group, consistent with group colours in [5]; in Replace/Split mode it splits into Test String \| Result |
| [7] Explanation ─ Code ─ Compare | Explanation tree; Code = a ready-to-paste snippet for the current flavor, mode and flags; Compare = the pattern run through all four engines side by side, with a "why / try" hint where one fails |

Interaction:

- Every action has a key. A bottom bar always shows the keys relevant to
  the focused panel (lazygit style). `?` opens a keybindings popup listing
  local keys first, then global, with `/` to filter (board 9).
- Global keys, as drawn: `1`–`7` jump to panel, `tab`/`shift-tab` cycle
  panels, `[`/`]` switch tabs, `:` command prompt (`:flavor go`,
  `:flags gim`, …), `ctrl-s` save to Library, `q` quit, `+`/`_` resize the
  focused panel. Keys that would type text while an editor is focused only
  apply outside editors; editors use `esc` to leave.
- **Correction to the mockups:** they show `ctrl-m` for mouse toggle; that
  is the same code as Enter in terminals and must not be used. Mouse is
  toggled with `:mouse` and a config setting. The builder should check the
  full keymap for other terminal-control collisions (`ctrl-m`, `ctrl-i`,
  `ctrl-[`, `ctrl-s`/`ctrl-q` flow control on some terminals) and adjust.
- **Mouse:** on by default (my call, like lazygit), off via config or
  `:mouse`. Click focuses panels, picks rows, switches tabs, and selects a
  pattern token; wheel scrolls; drag resizes the left column. Turning it
  off gives back the terminal's native text selection.
- Switching flavor keeps pattern, text and replacement; the new flavor
  re-evaluates; anything it can't parse is flagged, never changed.
- Undo/redo in the editors.
- Colours: dark theme as in the mockups; must stay readable in light
  terminals and in 256-colour terminals (builder picks the approach).

Code tab snippet languages (my call): Python → `re`; JavaScript → plain JS;
PCRE → PHP `preg_*`; Go → `regexp`. Snippet reflects mode (match / replace
/ split) and flags.

## Library and config

- Library: saved patterns (name, flavor, flags, pattern, replacement, test
  text) stored as a human-editable file in the OS config dir
  (`os.UserConfigDir()/lazyregex/`). Format is the builder's call.
- Config file in the same dir: default flavor, mouse on/off, timeout,
  theme overrides, keybinding overrides (keybinding overrides optional for
  v1).
- Session restore (last pattern/text/flavor) — builder's call; nice to
  have.

## Command line

As drawn on the help board (my additions; cheap and useful):

- `lazyregex` — open the TUI.
- `lazyregex -f go -p '\d+'` — open with flavor and pattern preset.
- `cat file.log | lazyregex` — piped stdin becomes the test string.
- `lazyregex --explain -f python '<pattern>'` — print the explanation tree
  and exit (non-interactive).
- `--version`, `--licenses`.

## Releasing

All from one GoReleaser config, triggered by pushing a `v*` tag (replacing
the current Poetry-based `release.yaml`):

- GitHub Release with archives + checksums for all platforms.
- **Homebrew tap** `dor1202/homebrew-lazyregex`: GoReleaser overwrites the
  existing Python/virtualenv formula with a binary formula. Needs a
  `HOMEBREW_TAP_GITHUB_TOKEN` secret with push access to the tap repo.
- **Scoop** bucket (new repo, e.g. `dor1202/scoop-lazyregex`).
- **Chocolatey** package (needs a Chocolatey API key secret; GoReleaser's
  chocolatey publisher runs on Windows — use a Windows job or publish
  manually for the first release; builder's call).
- **.deb / .rpm** via nfpm, attached to the release; **AUR** `lazyregex-bin`
  (needs an AUR SSH key secret). (My call, offered to the user without
  objection.)
- **Gating (my call):** v1.0.0 ships as soon as GitHub Releases, the
  Homebrew tap and .deb/.rpm work. Scoop, AUR and Chocolatey are added in
  follow-up releases as their accounts/secrets are ready (Chocolatey has a
  multi-day moderation queue). The maintainer creates the accounts, repos
  and secrets; the builder documents exactly which are needed.
- Expected binary size: roughly 30–40 MB, dominated by the CPython WASI
  module. Acceptable; trimming the bundled stdlib to what `re` needs is
  worthwhile.

The engine WASM artifacts are built reproducibly (a Makefile/Dockerfile
target that compiles pinned PCRE2/QuickJS/CPython sources with wasi-sdk)
and committed or fetched at build time with checksums — builder's call;
`go build` from a clean clone must work without a C toolchain.

## Testing

- **Parity tests are the core suite.** A corpus of (pattern, flags, text,
  mode) cases per flavor, including Unicode and edge cases. In CI, expected
  results are produced by the *real* native runtimes (CPython, Node, the
  `pcre2` library, Go) and compared to what lazyregex's bundled engines
  return: matches, spans, group names, replacement output, split output,
  and error/no-error. CI may install those runtimes; users never need them.
- Explainer tests per flavor (token tree snapshots).
- TUI tests for key flows (bubbletea `teatest` or equivalent): switching
  flavor, editing, Replace mode, help popup, mouse-off operation.
- Timeout test with a known catastrophic pattern for each backtracking
  engine.

## Migration in the repo

- Work on branch `go-rewrite`; remove the Python package, Poetry files,
  Python CI; add Go module, new CI (lint, test, parity), GoReleaser.
- Rewrite README: new screenshots/GIF, install instructions for each
  package manager, flavor/engine table with versions, keybindings.
- Note in README that v0.x was the Python version.

## Left to the builder

- TUI library (Bubble Tea + Lip Gloss is my suggestion; `gocui` as lazygit
  uses or `tview` are fine if they deliver the drawn layout and mouse
  support).
- How the explainer parsers are built.
- Exact engine versions, WASM build tooling, compilation-cache location.
- Library/config file formats.
- Colour palette details, light-terminal handling.
- Exact keymap beyond what's listed, subject to "every action has a key"
  and no terminal-control collisions.
