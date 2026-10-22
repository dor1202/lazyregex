# lazyregex v1.0 Go Rewrite Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the Python/Textual lazyregex with a single static Go binary: a lazygit-style regex tester whose Python, JavaScript, PCRE and Go flavors run on the real engines bundled inside it.

**Architecture:** One `engine.Engine` interface with four adapters. Go uses the stdlib `regexp`. PCRE2, QuickJS and CPython are WASI command modules run in-process by wazero. Each one runs a request loop speaking newline-delimited JSON over stdin/stdout. A `runner` layer handles lazy loading, warm-up, timeouts and cancellation. On top sit a flavor-aware explainer, a code-snippet generator, config/library storage, a Bubble Tea TUI and a small CLI. GoReleaser ships everything.

**Tech Stack:** Go 1.27, wazero (pure-Go WASM), wasi-sdk + Docker (engine builds only), PCRE2, QuickJS-ng, CPython 3.14 WASI, Bubble Tea + Lip Gloss + Bubbles (latest stable majors), GoReleaser + nfpm, GitHub Actions.

**Spec:** `docs/superpowers/specs/2026-10-20-go-rewrite-design.md`

## Global Constraints

- Module path `github.com/dor1202/lazyregex`; binary name `lazyregex`; first release tag `v1.0.0`.
- `CGO_ENABLED=0` for every build; `go build ./cmd/lazyregex` from a clean clone must work with no C toolchain and no network beyond Go modules.
- Each flavor's results come from that language's real engine; no emulation layer. Not acceptable: `dlclark/regexp2`, `goja`, or translating patterns between flavors.
- Never use a `python3`/`node`/`php` installed on the user's machine at runtime.
- No network access at runtime, including telemetry and update checks.
- Engine errors are shown with the engine's own message, unedited; hints are shown separately.
- Spans are shown in each language's native units: Python code points, JS UTF-16 code units, PCRE and Go bytes.
- Default flavor: Python. Default eval timeout: 2 s (configurable). Live-update cut-off: test text above 1 MiB (1,048,576 bytes) evaluates on `enter` only.
- Every action reachable by keyboard; mouse on by default, toggled with `:mouse` or config; never bind `ctrl-m`, `ctrl-i`, `ctrl-[`, `ctrl-s`, `ctrl-q` (terminal control/flow-control collisions; this overrides the spec's `ctrl-s` save, see Task 15).
- Switching flavor never edits the user's pattern; rewrites are applied only on an explicit key (`a`).
- Bundled licences (PSF for CPython, BSD for PCRE2, MIT for QuickJS-ng and cJSON) ship in release archives and print with `--licenses`.
- Platforms: darwin/amd64, darwin/arm64, linux/amd64, linux/arm64, windows/amd64, windows/arm64.

## Review Focus

1. **Non-ASCII text across flavors.** For a test string like `héllo 😀 wörld`, highlights must land on the same characters in all four flavors even though the span numbers differ. Pinned by `TestToByteOffsetsAllUnits` (Task 2) and parity corpus `unicode.yaml` (Tasks 3, 5–7).
2. **Empty matches and `g`.** Patterns like `a*` or `\b` on `"baaa"` must give exactly the native engine's match list and must not loop forever. Pinned by parity corpus `empty.yaml` (Task 3) and `TestRunnerEmptyMatchesTerminate` (Task 8).
3. **Typing quickly while a slow pattern runs.** A catastrophic pattern such as `(a+)+$` on `"aaaa…!"` must be cancelled by the next keystroke, and the engine must come back. Pinned by `TestHostTimeoutRestarts` (Task 4) and `TestRunnerCancelsSupersededEval` (Task 8).
4. **Unmatched optional groups.** `(a)?(b)` on `"b"` must show group 1 as "not matched", not as an empty string at 0. Pinned by `TestGoEngineUnmatchedGroup` (Task 2) and parity corpus `groups.yaml`.
5. **Piped binary or huge input.** `head -c 2000000 /dev/urandom | lazyregex` must open with a warning, U+FFFD replacements and manual evaluation, not crash or freeze. Pinned by `TestPrepareText*` (Task 8) and `TestCLIStdinBinary` (Task 18).

---

## File Structure

```
cmd/lazyregex/main.go             CLI entry: flags, stdin, --explain, --version, --licenses, starts TUI
internal/version/version.go       Version string (set by -ldflags)
internal/engine/engine.go         Flavor, Flag, Mode, Request, Result, Match, Group, Span, Unit, CompileError, Engine
internal/engine/offsets.go        native-unit ↔ byte offset conversion
internal/engine/goengine/         Go stdlib adapter
internal/engine/wasmhost/         wazero host: load gz module, NDJSON loop, timeout→restart, compile cache
internal/engine/pcre/             PCRE adapter + embedded pcre.wasm.gz
internal/engine/js/               QuickJS adapter + embedded qjs.wasm.gz + shim.js
internal/engine/python/           CPython adapter + embedded python.wasm.gz + stdlib.zip + shim.py
internal/engine/protocol/         shared JSON request/response wire types
internal/runner/                  lazy load, warm-up, cancellation, compare-all, PrepareText
internal/explain/                 AST, per-flavor dialect parser, describer → rows
internal/hints/                   why/try hints + rewrite suggestions
internal/codegen/                 snippets per flavor × mode
internal/config/                  config + library files in os.UserConfigDir()/lazyregex
internal/ui/                      Bubble Tea app, one file per panel, keys, help, prompt, mouse, theme
wasm/                             reproducible engine builds (Dockerfile, shims, versions.env, Makefile)
parity/                           corpus/*.yaml, gen/ (python, node, php, go generators), expected/, exceptions.yaml
.goreleaser.yaml, .github/workflows/{ci,parity,release}.yaml, Makefile, README.md, LICENSES/
```

---

### Task 1: Replace the Python project with a Go skeleton

**Files:**
- Delete: `lazyregex/`, `tests/`, `pyproject.toml`, `poetry.toml`, `.vscode/`, `.github/workflows/test.yaml`, `.github/workflows/release.yaml`
- Create: `go.mod`, `cmd/lazyregex/main.go`, `cmd/lazyregex/main_test.go`, `internal/version/version.go`, `Makefile`, `.github/workflows/ci.yaml`, `.gitignore` (Go)
- Keep: `LICENSE`, `README.md` (rewritten in Task 19), `assets/`, `.github/ISSUE_TEMPLATE/`

**Interfaces:**
- Produces: `version.Version string` (default `"dev"`, set via `-ldflags "-X github.com/dor1202/lazyregex/internal/version.Version=..."`); `func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int` in `cmd/lazyregex/main.go`, with `main` calling `os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))`.

- [ ] **Step 1: Remove the Python sources and tooling listed above** (`git rm -r`).
- [ ] **Step 2: `go mod init github.com/dor1202/lazyregex`**, with `go 1.27` in go.mod.
- [ ] **Step 3: Write the failing test**

```go
func TestVersionFlag(t *testing.T) {
	var out bytes.Buffer
	code := run([]string{"--version"}, strings.NewReader(""), &out, io.Discard)
	if code != 0 { t.Fatalf("exit %d", code) }
	if !strings.HasPrefix(out.String(), "lazyregex dev") { t.Fatalf("got %q", out.String()) }
}
```

- [ ] **Step 4: Run** `go test ./cmd/...`. Expected: FAIL (`run` undefined).
- [ ] **Step 5: Implement `run`** with the stdlib `flag` package (`flag.NewFlagSet("lazyregex", flag.ContinueOnError)`). `--version` prints `lazyregex <Version>\n`. With no flags, print `TUI not implemented` for now and return 0.
- [ ] **Step 6: Run** `go test ./...`. Expected: PASS.
- [ ] **Step 7: Add the Makefile** with targets `build` (`CGO_ENABLED=0 go build -o bin/lazyregex ./cmd/lazyregex`), `test` (`go test ./...`), `lint` (`golangci-lint run`), and `wasm` (filled in by Tasks 5–7). Add `ci.yaml`: on push/PR, a matrix of ubuntu-latest, macos-latest and windows-latest running `go vet ./...` and `go test ./...`, plus a golangci-lint job on ubuntu.
- [ ] **Step 8: Commit** `chore: replace Python implementation with Go skeleton`.

---

### Task 2: Engine core types and the Go engine

**Files:**
- Create: `internal/engine/engine.go`, `internal/engine/offsets.go`, `internal/engine/offsets_test.go`, `internal/engine/goengine/goengine.go`, `internal/engine/goengine/goengine_test.go`

**Interfaces:**
- Produces (in `engine`):

```go
type Flavor string // "python" | "js" | "pcre" | "go"
const (Python Flavor = "python"; JS Flavor = "js"; PCRE Flavor = "pcre"; Go Flavor = "go")
var Flavors = []Flavor{Python, JS, PCRE, Go} // panel order

type Mode string // "match" | "replace" | "split"
type Unit int    // UnitBytes | UnitCodePoints | UnitUTF16

type Span struct{ Start, End int }            // native units
type Group struct {
	Index   int
	Name    string // "" if unnamed
	Matched bool   // false => Span is meaningless
	Span    Span
}
type Match struct{ Span Span; Groups []Group } // Groups excludes group 0
type Request struct {
	Pattern string
	Flags   string // flavor flag letters, e.g. "gm"; "g" is lazyregex's global
	Text    string
	Mode    Mode
	Replacement string // replace mode
}
type Result struct {
	Matches []Match
	Output  string   // replace result
	Parts   []string // split result; nil for an unmatched optional group in a Python split (rendered "None")
	PartsNil []bool  // parallel to Parts: true where the native value was None/undefined
	Unit    Unit
}
type CompileError struct{ Message string; Offset int } // Offset -1 if engine gives none; Message verbatim
func (e *CompileError) Error() string

type Info struct{ Flavor Flavor; EngineName, Version string; Flags []FlagInfo }
type FlagInfo struct{ Letter byte; Name string; Default bool }

type Engine interface {
	Info() Info
	Eval(ctx context.Context, req Request) (Result, error) // *CompileError for bad patterns; ctx.Err() on cancel/timeout
	Close() error
}
func ToByteOffsets(text string, unit Unit, offsets []int) []int // offsets in unit → byte offsets in text
```

- Produces (in `goengine`): `func New() engine.Engine`. Flags `g i m s U`, with `g` default on. `i m s U` become an inline `(?flags)` prefix. A `CompileError` has its `Offset` taken from `regexp/syntax.Error.Expr`'s position in the pattern when one can be located, else -1. The error message is `err.Error()` exactly.

- [ ] **Step 1: Write the failing offset tests.** `TestToByteOffsetsAllUnits` uses text `"héllo 😀 wörld"`. Code point offsets `[0,1,6,7,8]` map to bytes `[0,1,7,11,12]`. UTF-16 offsets `[0,1,6,8,9]` map to bytes `[0,1,7,11,12]`. Byte offsets are returned unchanged. An offset equal to the length maps to `len(text)`.
- [ ] **Step 2: Write the failing Go engine tests.**
  - `TestGoEngineFindAll`: pattern `(?P<y>\d{4})-(\d\d)`, flags `g`, text `"2026-10 and 1999-01"` gives 2 matches. Match 0 has span {0,7} and groups `[{1,"y",true,{0,4}},{2,"",true,{5,7}}]`. Unit is `UnitBytes`.
  - `TestGoEngineNonGlobal`: the same request without `g` gives 1 match.
  - `TestGoEngineUnmatchedGroup`: `(a)?(b)` on `"b"` gives group 1 with `Matched:false`.
  - `TestGoEngineCompileError`: `(?<=\$)\d+` gives `*CompileError` with Message `` "error parsing regexp: invalid named capture: `(?<=\$)\d+`" `` (the Go 1.27 wording, verified; the mockup shows an older message).
  - `TestGoEngineReplace`: `(\w+)@(\w+)` with replacement `$2 at $1`, flags `g`, on `"a@b c@d"` gives `"b at a d at c"`. Without `g`, only the first match is replaced, giving `"b at a c@d"`.
  - `TestGoEngineSplit`: `,\s*` on `"a, b,c"` gives `["a","b","c"]`.
  - `TestGoEngineFlags`: `(?i)` behaviour via flag `i`: `abc` on `"ABC"` matches.
- [ ] **Step 3: Run** `go test ./internal/engine/...`. Expected: FAIL (undefined).
- [ ] **Step 4: Implement `engine.go`, `offsets.go`, `goengine.go`.**
  - Global matching uses `FindAllStringSubmatchIndex(text, -1)`; non-global uses `FindStringSubmatchIndex`.
  - Replace with `g` uses `ReplaceAllString`. Non-global replace finds the first match, runs `Expand`, and splices the result in.
  - Split uses `re.Split(text, -1)`.
  - Every engine checks `ctx.Err()` before running.
- [ ] **Step 5: Run** `go test ./internal/engine/...`. Expected: PASS.
- [ ] **Step 6: Commit** `feat(engine): core types and Go stdlib engine`.

---

### Task 3: Parity harness

**Files:**
- Create: `parity/corpus/{basic,groups,unicode,empty,flags,replace,split,errors}.yaml`, `parity/gen/gen_python.py`, `parity/gen/gen_node.mjs`, `parity/gen/gen_php.php`, `parity/gen/gen_go/main.go`, `parity/expected/<flavor>/<corpus>.json` (generated, committed), `parity/exceptions.yaml`, `parity/parity_test.go`, `.github/workflows/parity.yaml`

**Interfaces:**
- Consumes: `engine.Engine`, `engine.Request`, `engine.Result`.
- Produces:
  - **Corpus case format:** `{id, flavors: [python, js, pcre, go] (default all), pattern, flags, text, mode, replacement}`.
  - **Expected file format:** a JSON array of `{id, ok, error_message?, matches:[{span:[s,e], groups:[{index,name,matched,span}]}], output?, parts?, parts_nil?, unit}`.
  - `func RunParity(t *testing.T, flavor engine.Flavor, e engine.Engine)` in `parity/parity_test.go`. Later tasks call it from their own `TestParity` by importing the `parity` package helpers. Put the reusable part in `parity/parity.go` and keep the test in `_test.go`.
  - **`exceptions.yaml`:** `[{flavor, id, field: matches|error_message|output|parts, reason}]`.

Generators run the native runtime, which CI installs. For each case:

- **Python** (`gen_python.py`, run with python 3.14): non-`g` uses `re.search`, `g` uses `re.finditer`; replace uses `re.sub(count=0 if g else 1)`; split uses `re.split`. Spans come from `m.span(i)`, and `m.span(i) == (-1,-1)` means not matched. Errors are the `str(e)` of `re.error`.
- **Node** (`gen_node.mjs`): the flags string is passed straight to `new RegExp(p, flags + "d")` (the `d` is added only for indices). Non-`g` uses `exec`, `g` uses `matchAll`; replace uses `str.replace`; split uses `str.split`. Errors are `e.message`.
- **PHP** (`gen_php.php`, run with PHP 8.4): the pattern is wrapped as `"/" . addcslashes($p, "/") . "/" . $modifiers`, where the modifiers are the flags minus `g`. Non-`g` uses `preg_match`, `g` uses `preg_match_all` with `PREG_SET_ORDER|PREG_OFFSET_CAPTURE|PREG_UNMATCHED_AS_NULL`; replace uses `preg_replace(limit: g ? -1 : 1)`; split uses `preg_split`. Errors come from `preg_last_error_msg()` plus the warning text captured with `set_error_handler`.
- **Go**: calls `goengine` directly.

- [ ] **Step 1: Write the corpora.** Use at least 15 cases per file. They must include the review-focus cases: `unicode.yaml` with `héllo 😀 wörld` and `\w+`, `.`, `\b`; `empty.yaml` with `a*` on `"baaa"`, `\b` on `"ab cd"`, `x*` on `""`, `(?=a)` on `"aa"`; `groups.yaml` with `(a)?(b)` on `"b"`, named groups and nested groups; `errors.yaml` with an unbalanced `(`, `(?<=\$)` (errors in Go only), and `\k<x>` without a group.
- [ ] **Step 2: Write the four generators** to the specs above, and generate `expected/go/*.json`. The other flavors' expected files are generated in CI and committed by Tasks 5–7.
- [ ] **Step 3: Write `TestParityGo`**, which calls `RunParity(t, engine.Go, goengine.New())`. `RunParity` loads the expected JSON, runs each case, and compares field by field (converting none of the spans, since both sides are native). It skips `(flavor, id, field)` entries listed in `exceptions.yaml`, logs them, and fails on everything else with a diff that names the case id.
- [ ] **Step 4: Run** `go test ./parity/ -run TestParityGo`. Expected: PASS.
- [ ] **Step 5: Add `parity.yaml` CI.** It sets up python 3.14, node 24 and php 8.4 (`shivammathur/setup-php`), runs all four generators into a temp dir, and `diff -r`s against `parity/expected`. A difference means the committed fixtures are stale, so the job fails. Then it runs `go test ./parity/...`.
- [ ] **Step 6: Commit** `test(parity): corpus, native generators and Go parity`.

---

### Task 4: wazero host for engine modules

**Files:**
- Create: `internal/engine/protocol/protocol.go`, `internal/engine/wasmhost/host.go`, `internal/engine/wasmhost/host_test.go`, `internal/engine/wasmhost/testdata/echo/main.go`

**Interfaces:**
- Produces (`protocol`): the wire types, one JSON object per line.

```go
type Req struct {
	ID int `json:"id"`; Op string `json:"op"` // "eval" | "ping"
	Pattern, Flags, Text, Mode, Replacement string `json:"..."` // snake_case tags
}
type Resp struct {
	ID int; OK bool
	Error *struct{ Message string; Offset int }
	Matches []struct{ Span [2]int; Groups []struct{ Index int; Name string; Matched bool; Span [2]int } }
	Output string; Parts []string; PartsNil []bool
	Version string // on "ping"
}
func ToResult(r Resp, unit engine.Unit) (engine.Result, error) // Error → *engine.CompileError
```

- Produces (`wasmhost`):

```go
type Spec struct {
	Name     string   // "pcre"
	ModuleGZ []byte   // gzip'd wasm (embedded by adapters)
	Args     []string // argv, e.g. {"python", "-I", "-S", "/shim/shim.py"}
	Env      map[string]string
	Mounts   map[string]fs.FS // guest path → fs
}
type Host struct{ /* one live instance, restartable */ }
func New(ctx context.Context, spec Spec, cacheDir string) (*Host, error) // compiles (cached via wazero.NewCompilationCacheWithDir), starts instance, sends ping
func (h *Host) Call(ctx context.Context, req protocol.Req) (protocol.Resp, error)
func (h *Host) Version() string
func (h *Host) Close() error
```

Behaviour:
- The module runs as a WASI command in a goroutine, with stdin/stdout connected through `io.Pipe`s and `wazero.NewRuntimeConfig().WithCloseOnContextDone(true)`.
- `Call` writes one JSON line and reads one line.
- If `ctx` ends first, the host closes the module and returns `ctx.Err()`. The next `Call` transparently starts a fresh instance, which is the restart.
- Calls are serialized with a mutex.
- Module stderr is captured into a 64 KiB ring buffer. It goes into the error if the instance exits unexpectedly.

- [ ] **Step 1: Write `testdata/echo/main.go`.** It is a `wasip1` program that answers `ping` with `{"id":N,"ok":true,"version":"echo-1"}`. On `eval` it returns `ok:true, output: <text uppercased>`; if `pattern == "hang"` it loops forever.
- [ ] **Step 2: Write the failing tests.** A `TestMain` helper builds echo with `GOOS=wasip1 GOARCH=wasm go build -o $TMP/echo.wasm`, gzips it, and skips the tests if `go` is not on PATH.
  - `TestHostEval`: `Call` with text `"abc"` returns output `"ABC"`. `Version()` returns `"echo-1"`.
  - `TestHostTimeoutRestarts`: `Call` with pattern `"hang"` and a 200 ms context returns `context.DeadlineExceeded` within 1 s. The next `Call` with text `"x"` returns `"X"`.
  - `TestHostCompileCache`: a second `New` with the same cacheDir starts at least 2× faster than the first. Log the timings; assert only that the cache directory is non-empty.
- [ ] **Step 3: Run** `go test ./internal/engine/wasmhost/`. Expected: FAIL.
- [ ] **Step 4: Implement** `protocol.go` and `host.go` to the behaviour above.
- [ ] **Step 5: Run** the tests again. Expected: PASS.
- [ ] **Step 6: Commit** `feat(engine): wazero host with NDJSON protocol and restart-on-timeout`.

---

### Task 5: PCRE engine (PHP semantics)

**Files:**
- Create: `wasm/versions.env` (`PCRE2_VERSION`, `QUICKJS_NG_VERSION`, `CPYTHON_VERSION`, `WASI_SDK_VERSION`; pin the latest stable releases and record the tarball SHA-256s), `wasm/Dockerfile`, `wasm/pcre/shim.c`, `wasm/pcre/cJSON.{c,h}` (vendored, MIT), `wasm/pcre/build.sh`, `internal/engine/pcre/pcre.go`, `internal/engine/pcre/pcre.wasm.gz` (built artifact, committed), `internal/engine/pcre/pcre_test.go`, `parity/expected/pcre/*.json`
- Modify: `Makefile` (`wasm-pcre` target runs `docker build` and copies the artifact out)

**Interfaces:**
- Consumes: `wasmhost.New/Call`, `protocol.ToResult`.
- Produces: `func New(ctx context.Context, cacheDir string) (engine.Engine, error)`.
  - Info: EngineName `"PCRE2"`, Version taken from the ping (`pcre2_config(PCRE2_CONFIG_VERSION)`).
  - Flags: `g i m s x n U u`, with `g` and `u` on by default.
  - Unit: `UnitBytes`.

The shim is a WASI C command that reads lines and replicates PHP's `ext/pcre/php_pcre.c` (PHP 8.4) behaviour:
- Modifiers map as follows: `i→PCRE2_CASELESS`, `m→MULTILINE`, `s→DOTALL`, `x→EXTENDED`, `n→NO_AUTO_CAPTURE`, `U→UNGREEDY`, `u→PCRE2_UTF|PCRE2_UCP` plus UTF validity checking.
- Iteration for `preg_match_all` uses PHP's empty-match retry: after an empty match, retry at the same offset with `PCRE2_NOTEMPTY_ATSTART|PCRE2_ANCHORED`, else advance one character (one UTF-8 sequence in `u` mode).
- Replacement expansion follows `preg_replace`: `\n`, `$n`, `${n}` with n of at most 2 digits, and nothing else.
- Split follows `preg_split` with limit -1 and no flags.
- Compile errors use PHP's message format: `preg_match(): Compilation failed: <pcre2 message> at offset <n>`.
- Read the PHP source while writing the shim. The parity fixtures from Task 3 are the referee.

- [ ] **Step 1: Write `wasm/Dockerfile`** (wasi-sdk image, verified source downloads) and `build.sh`. Build PCRE2 as a static `libpcre2-8.a` (no JIT) and link `shim.c` into `pcre.wasm`. `make wasm-pcre` writes `internal/engine/pcre/pcre.wasm.gz`.
- [ ] **Step 2: Write the failing tests.**
  - `TestPCREPing` checks that Version starts with `"10."`.
  - `TestPCRELookbehind`: `(?<=\$)\d+(\.\d{2})?`, flags `gu`, on `"Total: $42.50, tax $3.10"` gives spans {8,13} and {20,24}.
  - `TestParityPCRE` calls `parity.RunParity(t, engine.PCRE, e)`.
- [ ] **Step 3: Run** `go test ./internal/engine/pcre/`. Expected: FAIL.
- [ ] **Step 4: Write `shim.c` and `pcre.go`.** Embed the module with `//go:embed pcre.wasm.gz`.
- [ ] **Step 5: Generate `parity/expected/pcre/*.json`** with `gen_php.php` (in Docker `php:8.4-cli` if PHP is not installed locally). Then run `go test ./internal/engine/pcre/ ./parity/...`. Expected: PASS. Any remaining diff is a shim bug to fix, never an exception.
- [ ] **Step 6: Commit** `feat(engine): PCRE2 engine with PHP preg semantics`.

---

### Task 6: JavaScript engine (QuickJS-ng)

**Files:**
- Create: `wasm/quickjs/build.sh`, `internal/engine/js/shim.js`, `internal/engine/js/js.go`, `internal/engine/js/qjs.wasm.gz`, `internal/engine/js/js_test.go`, `parity/expected/js/*.json`
- Modify: `Makefile` (`wasm-js`), `parity/exceptions.yaml`, `wasm/Dockerfile`

**Interfaces:**
- Produces: `func New(ctx context.Context, cacheDir string) (engine.Engine, error)`.
  - Info: EngineName `"QuickJS-ng"`.
  - Flags: `g d i m s u v y`, with `g` default on.
  - Unit: `UnitUTF16`.
- Host args: `{"qjs", "--std", "/shim/shim.js"}`, with `shim.js` mounted from an `embed.FS`.
- **shim.js** reads lines with `std.in.getline()`, parses JSON, builds `new RegExp(pattern, flags + (flags.includes("d") ? "" : "d"))`, and uses `exec` / `matchAll` / `String.prototype.replace` / `split` the same way `gen_node.mjs` does. Errors are `e.message`. Ping returns the QuickJS-ng version string.

- [ ] **Step 1: Build `qjs.wasm`** from the pinned QuickJS-ng with its WASI build (`cmake -DCMAKE_TOOLCHAIN_FILE=<wasi-sdk>/share/cmake/wasi-sdk.cmake`). `make wasm-js` writes `qjs.wasm.gz`.
- [ ] **Step 2: Write the failing tests.**
  - `TestJSUnicodeIndices`: `.` with flags `gu` on `"a😀b"` gives spans {0,1}, {1,3}, {3,4} (UTF-16).
  - `TestJSNamedGroup`: `(?<y>\d{4})` gives group name `"y"`.
  - `TestParityJS` calls `RunParity`.
- [ ] **Step 3: Run.** Expected: FAIL.
- [ ] **Step 4: Implement `shim.js` and `js.go`.**
- [ ] **Step 5: Generate the Node fixtures and run parity.**
  - Every remaining mismatch must be a documented QuickJS vs V8 difference: error wording, Unicode data version, or `v`-flag edges. Add each one to `exceptions.yaml` with a `reason`. Mismatches in matches, output or parts outside those categories are bugs to fix.
  - Expected: PASS.
- [ ] **Step 6: Commit** `feat(engine): JavaScript engine on QuickJS-ng`.

---

### Task 7: Python engine (CPython WASI)

**Files:**
- Create: `wasm/python/build.sh`, `wasm/python/stdlib-allowlist.txt`, `internal/engine/python/shim.py`, `internal/engine/python/python.go`, `internal/engine/python/python.wasm.gz`, `internal/engine/python/stdlib.zip`, `internal/engine/python/python_test.go`, `parity/expected/python/*.json`
- Modify: `Makefile` (`wasm-python`), `wasm/Dockerfile`

**Interfaces:**
- Produces: `func New(ctx context.Context, cacheDir string) (engine.Engine, error)`.
  - Info: EngineName `"CPython re"`, Version from `sys.version.split()[0]`.
  - Flags: `g i m s x a`, with `g` default on.
  - Unit: `UnitCodePoints`.
- Host:
  - Args `{"python", "-I", "-S", "-u", "/shim/shim.py"}`, env `PYTHONPATH=/lib/stdlib.zip`, `PYTHONHOME=/`.
  - Mounts: `/lib` (stdlib.zip, stored uncompressed so zipimport works without zlib) and `/shim`.
- **shim.py** uses the same semantics as `gen_python.py` (share the case-running function by copying it, and keep the two in sync with a test, Step 2). It converts flag letters to `re` flags (`i→I m→M s→S x→X a→A`), and `g` selects `finditer` vs `search`.

- [ ] **Step 1: Build CPython `python.wasm`** with `Tools/wasm/wasi` for the pinned 3.14.x. Build `stdlib.zip` from the allowlist: `re/`, `enum`, `functools`, `types`, `operator`, `keyword`, `reprlib`, `collections/`, `copyreg`, `json/`, `encodings/` (utf_8, aliases, latin_1, ascii), `codecs`, `io`, `abc`, `_collections_abc`, `_weakrefset`, `weakref`, `sys`-free bits only. Grow the list until `-c "import re, json"` runs. `make wasm-python` writes both artifacts.
- [ ] **Step 2: Write the failing tests.**
  - `TestPythonPing`: the version starts with `"3.14."`.
  - `TestPythonNamedGroups`: `(?P<date>\d{4}-\d{2}-\d{2}) (?P<level>INFO|WARN|ERROR)` on `"2026-10-20 INFO  x"` gives date {0,10}, level {11,15}.
  - `TestPythonCodePoints`: `😀(b)` on `"a😀b"` gives the match {1,3} and group {2,3}.
  - `TestPythonErrorVerbatim`: `(` gives a Message equal to Python's `str(re.error)`: `"missing ), unterminated subpattern at position 0"`.
  - `TestShimMatchesGenerator`: the case-running function bodies in `shim.py` and `parity/gen/gen_python.py` are byte-identical between `# --- shared ---` markers.
  - `TestParityPython` calls `RunParity`.
- [ ] **Step 3: Run.** Expected: FAIL.
- [ ] **Step 4: Implement `shim.py` and `python.go`.**
- [ ] **Step 5: Generate the fixtures with python 3.14 and run parity.** Expected: PASS with zero exceptions.
- [ ] **Step 6: Record cold-start numbers in the commit message:** first `New` with an empty cache, then with a warm cache. They inform Task 8's loading UI.
- [ ] **Step 7: Commit** `feat(engine): Python engine on CPython 3.14 WASI`.

---

### Task 8: Runner (lazy load, warm-up, cancellation, text preparation)

**Files:**
- Create: `internal/runner/runner.go`, `internal/runner/text.go`, `internal/runner/runner_test.go`, `internal/runner/text_test.go`

**Interfaces:**
- Consumes: the four `New` constructors, `engine.*`.
- Produces:

```go
type State int // NotLoaded | Loading | Ready | Restarting | Failed
type Factory func(ctx context.Context, cacheDir string) (engine.Engine, error)
type Runner struct{ /* ... */ }
func New(factories map[engine.Flavor]Factory, cacheDir string, timeout time.Duration) *Runner
func (r *Runner) State(f engine.Flavor) State
func (r *Runner) Info(f engine.Flavor) (engine.Info, bool)   // false until loaded
func (r *Runner) Load(f engine.Flavor)                       // async; no-op if loading/ready
func (r *Runner) WarmUp(except engine.Flavor)                // async Load of all others
func (r *Runner) Eval(f engine.Flavor, req engine.Request) (engine.Result, error)
	// cancels any in-flight Eval for the same flavor (returns context.Canceled to it),
	// waits for load, applies timeout; timeout error is ErrTimeout
func (r *Runner) EvalAll(req map[engine.Flavor]engine.Request) map[engine.Flavor]Outcome // only Ready flavors
type Outcome struct{ Result engine.Result; Err error }
func (r *Runner) Events() <-chan Event // Event{Flavor, State}
func (r *Runner) Close()
var ErrTimeout = errors.New("timed out — possible catastrophic backtracking")

// text.go
type Prepared struct{ Text string; InvalidUTF8 bool; Manual bool } // Manual: len > 1<<20
func PrepareText(b []byte) Prepared
```

Default cacheDir: `filepath.Join(os.UserCacheDir(), "lazyregex", "wazero")`.

- [ ] **Step 1: Write the failing tests** using fake factories (a fake engine whose `Eval` sleeps for `req.Pattern`-encoded durations and honours ctx).
  - `TestRunnerLazyLoad`: `State` is NotLoaded until `Load` or `Eval` is called.
  - `TestRunnerCancelsSupersededEval`: the first Eval (sleep 1 s) returns `context.Canceled` once a second Eval starts, and the second completes.
  - `TestRunnerTimeout`: a 50 ms timeout with a sleep of 1 s gives `ErrTimeout`, and the flavor emits `Restarting` then `Ready`.
  - `TestRunnerEvalAllOnlyReady`: flavors that aren't loaded are absent from the result.
  - `TestRunnerEmptyMatchesTerminate`: the real `goengine` with `a*` on `"baaa"` and `g` returns within 100 ms with 2 matches ({0,0}, {1,4}). This is Go's real behaviour, verified; Python gives 3 for the same input, which the parity corpus also pins.
  - `TestPrepareTextInvalidUTF8`: `[]byte{0x66, 0xff, 0x6f}` gives Text `"f�o"` and InvalidUTF8 true.
  - `TestPrepareTextLarge`: 1,048,577 bytes gives Manual true; 1,048,576 gives false.
- [ ] **Step 2: Run** `go test ./internal/runner/`. Expected: FAIL.
- [ ] **Step 3: Implement.** Use one goroutine plus a cancel func per flavor. Use `strings.ToValidUTF8(s, "�")`.
- [ ] **Step 4: Run.** Expected: PASS (also with `-race`).
- [ ] **Step 5: Commit** `feat(runner): lazy engines, warm-up, cancellation and text preparation`.

---

### Task 9: Explainer parser (AST with flavor dialects)

**Files:**
- Create: `internal/explain/ast.go`, `internal/explain/dialect.go`, `internal/explain/parse.go`, `internal/explain/parse_test.go`

**Interfaces:**
- Produces:

```go
type Kind int // Alternation, Sequence, Literal, Dot, CharClass, Shorthand (\d\w\s…), Anchor (^ $ \b \B \A \z \Z \G),
              // Group (Capture|Named|NonCapture|Atomic|LookAhead|NegLookAhead|LookBehind|NegLookBehind|InlineFlags|BranchReset),
              // Quantifier, Backref, NamedBackref, UnicodeProp, Recursion, KeepOut (\K), Comment, Unknown
type Node struct {
	Kind     Kind
	Span     [2]int // byte offsets in the pattern
	Raw      string
	Children []*Node
	// payload
	Name        string     // group/backref name, property name
	Index       int        // capture index / backref number
	Min, Max    int        // quantifier; Max -1 = unbounded
	Greedy      string     // "greedy" | "lazy" | "possessive"
	Negated     bool       // classes, \D, \P
	Flags       string     // inline flags on/off, e.g. "i-m"
	ClassItems  []ClassItem // ranges/literals/shorthands inside [...]
}
type ClassItem struct{ Lo, Hi rune; Shorthand string; Prop string; Negated bool }
type Dialect struct {
	Flavor engine.Flavor
	NamedGroup   []string // python: "(?P<", "(?<"(3.?)… — encode exactly what each flavor accepts
	NamedBackref []string // python "(?P=name)", js "\k<name>", pcre "\k<name>" "\k'name'" "(?P=name)" "\g{name}", go: none
	Lookbehind, Atomic, Possessive, Recursion, KeepOut, BranchReset, UnicodeProps, Backrefs bool
}
func DialectFor(f engine.Flavor, flags string) Dialect // x flag enables comments/whitespace for python/pcre
func Parse(pattern string, d Dialect) *Node // never fails; unparseable pieces become Unknown nodes with their span
```

Dialect facts, which the tests pin:
- **Python 3.14:** atomic `(?>…)` and possessive quantifiers are supported (3.11+); there is no recursion and no `\K`; named groups are `(?P<n>…)` (`(?<n>` is a syntax error).
- **JS:** named groups `(?<n>…)`, lookbehind yes; no atomic, possessive, recursion or `\K`; `\p{…}` only with `u`/`v`.
- **PCRE:** everything listed.
- **Go:** `(?P<n>…)` and `(?<n>…)`; no lookaround, backrefs, atomic, possessive or recursion.

- [ ] **Step 1: Write the failing table-driven tests** (`TestParse`), one row per construct per flavor, asserting the Kind tree and spans. They must include:
  - `^(?P<date>\d{4})$` in Python gives Sequence[Anchor, Group(Named "date")[Quantifier{4,4}[Shorthand d]], Anchor].
  - `(?<=\$)\d+` in JS gives LookBehind; in Go, the `(?<=` piece is Unknown.
  - `a++` in PCRE and Python gives a possessive quantifier; in JS, the `+` after `a+` is Unknown.
  - `(?R)` in PCRE gives Recursion.
  - `[^a-z\d]` gives a negated CharClass with items `{a-z}` and `{\d}`.
  - `\p{L}` in JS with flags `u` gives UnicodeProp; without `u`, it is a Literal `p` (JS's Annex B behaviour, so not Unknown).
  - `(?i:abc)` gives InlineFlags with children.
  - `a|b|c` gives Alternation with 3 children.
- [ ] **Step 2: Run** `go test ./internal/explain/`. Expected: FAIL.
- [ ] **Step 3: Implement** a recursive-descent parser driven by `Dialect`.
- [ ] **Step 4: Run.** Expected: PASS.
- [ ] **Step 5: Commit** `feat(explain): flavor-aware regex parser`.

---

### Task 10: Explanation rows

**Files:**
- Create: `internal/explain/describe.go`, `internal/explain/describe_test.go`, `internal/explain/testdata/*.golden`

**Interfaces:**
- Consumes: `Parse`, `Node`.
- Produces:

```go
type Row struct {
	Depth int     // tree depth for ├─ └─ drawing
	Token string  // display form, groups collapsed as "(?P<date>…)"
	Text  string  // plain English
	Span  [2]int  // pattern byte span (for underline / click-to-select)
	Group int     // capture index this row belongs to (0 = none), for colouring
	Unknown bool  // "can't explain this part"
}
func Explain(pattern string, f engine.Flavor, flags string) []Row
func Render(rows []Row) string // plain text tree, used by --explain and golden tests
```

Wording follows the mockup:
- "start of a line (m flag)" or "start of the string"
- `named group "date"`
- "a digit, exactly 4 times"
- `"-" literally`
- "one of 3 alternatives"
- "whitespace, 1 or more (greedy)"
- "any char but newline, 0 or more (greedy)"
- "can't explain this part"

- [ ] **Step 1: Write the failing golden tests** `TestExplainGolden/<name>`, one golden file per case. They cover the mockup's log-line pattern (Python, `gm`, matching the rows on board 6), a PCRE pattern with `\K`, recursion and possessive, a JS pattern with `\p{L}` and `v`, and a Go pattern with an unsupported lookbehind (giving an Unknown row). An `-update` flag regenerates the files.
- [ ] **Step 2: Run.** Expected: FAIL.
- [ ] **Step 3: Implement `Explain` and `Render`.**
- [ ] **Step 4: Review the golden files by eye against the mockup wording, then run.** Expected: PASS.
- [ ] **Step 5: Commit** `feat(explain): plain-English explanation rows`.

---

### Task 11: Hints and code snippets

**Files:**
- Create: `internal/hints/hints.go`, `internal/hints/hints_test.go`, `internal/codegen/codegen.go`, `internal/codegen/codegen_test.go`, `internal/codegen/testdata/*.golden`

**Interfaces:**
- Produces:

```go
// hints
type Hint struct{ Why string; Rewrite string /* "" if none */; RewriteNote string }
func For(f engine.Flavor, pattern string, flags string, err *engine.CompileError) (Hint, bool)
// codegen
func Snippet(f engine.Flavor, req engine.Request) string
```

**Hint rules (v1):**
- For Go, find unsupported constructs through `explain.Parse` with Go's dialect, where Unknown nodes whose Raw starts with `(?<=`, `(?<!`, `(?=`, `(?!`, `\1`–`\9`, `(?>`, or a possessive quantifier get a Why text naming the construct and RE2's linear-time guarantee.
- Rewrite only in two cases: a leading positive lookbehind `(?<=A)B` becomes `A(B)` with the note `read group 1`, and a trailing positive lookahead `A(?=B)` becomes `(A)B` with the note `read group 1`.
- For a JS error caused by `(?P<`, the Why is `JavaScript uses (?<name>…) for named groups`, with that rewrite.
- No hint otherwise.

**Snippets:**
- Python: `re.compile(r"…", re.MULTILINE…)` plus `finditer` / `search` / `sub` / `split`. The raw string falls back to a normal escaped string when the pattern ends in an odd number of backslashes or contains `"`.
- JS: a `/…/flags` literal plus `matchAll` / `exec` / `replace` / `split`.
- PCRE (PHP): `preg_match_all('/…/mods', $text, $m)` / `preg_match` / `preg_replace` / `preg_split`, single-quoted with `'` and `\` escaped.
- Go: `regexp.MustCompile(`+"`…`"+`)` (falling back to a quoted string if the pattern contains a backtick), with the `(?flags)` prefix, plus `FindAllStringSubmatch` / `ReplaceAllString` / `Split`.

- [ ] **Step 1: Write the failing tests.**
  - `TestGoLookbehindRewrite`: `(?<=\$)\d+(\.\d{2})?` gives Rewrite `\$(\d+(\.\d{2})?)` and RewriteNote `read group 1`.
  - `TestNoHintForValid`.
  - `TestJSPythonNamedGroupHint`.
  - Codegen golden tests: 4 flavors × 3 modes, plus Python with a pattern ending in `\`.
- [ ] **Step 2: Run.** Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run.** Expected: PASS.
- [ ] **Step 5: Commit** `feat: why/try hints and code snippets`.

---

### Task 12: Config and library storage

**Files:**
- Create: `internal/config/config.go`, `internal/config/library.go`, `internal/config/config_test.go`

**Interfaces:**
- Produces:

```go
type Config struct {
	DefaultFlavor engine.Flavor `yaml:"default_flavor"` // default "python"
	Mouse         bool          `yaml:"mouse"`          // default true
	Timeout       time.Duration `yaml:"timeout"`        // default 2s
	Theme         map[string]string `yaml:"theme"`      // optional colour overrides by token name
}
func Dir() (string, error)                   // os.UserConfigDir()/lazyregex
func Load(dir string) (Config, error)        // missing file → defaults; unknown keys ignored
func Save(dir string, c Config) error
type Entry struct{ Name string; Flavor engine.Flavor; Flags, Pattern, Replacement, Text string }
type Library struct{ Entries []Entry }
func LoadLibrary(dir string) (Library, error) // library.yaml
func (l *Library) Put(e Entry)                // replace by Name or append
func (l *Library) Delete(name string)
func SaveLibrary(dir string, l Library) error  // atomic: write temp + rename
```

Files are `config.yaml` and `library.yaml` (`gopkg.in/yaml.v3`).

- [ ] **Step 1: Write the failing tests.**
  - `TestLoadDefaults`: an empty dir gives python, mouse true, 2 s.
  - `TestLoadPartial`: `mouse: false` alone keeps the other defaults.
  - `TestLibraryRoundTrip`: Put, Save, Load gives equal entries, and the text keeps newlines.
  - `TestLibraryPutReplaces`.
  - `TestSaveAtomic`: no temp files remain.
- [ ] **Step 2: Run.** Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run.** Expected: PASS.
- [ ] **Step 5: Commit** `feat(config): config and pattern library`.

---

### Task 13: TUI shell: layout, focus, panels, key bar

**Files:**
- Create: `internal/ui/app.go`, `internal/ui/layout.go`, `internal/ui/panel.go`, `internal/ui/theme.go`, `internal/ui/keys.go`, `internal/ui/keybar.go`, `internal/ui/app_test.go`
- Modify: `cmd/lazyregex/main.go` (start the TUI when stdout is a terminal)

**Interfaces:**
- Consumes: `runner.Runner`, `config.Config`, `config.Library`.
- Produces:

```go
type PanelID int // PFlavor=1, PFlags, PMatches, PLibrary, PPattern, PText, PDetails (=7)
type Deps struct {
	Runner  *runner.Runner; Config config.Config; ConfigDir string; Library config.Library
	Initial struct{ Flavor engine.Flavor; Pattern, Text string; TextPrepared runner.Prepared }
}
func New(d Deps) tea.Model
type Panel interface {
	Title() string               // "[3]─Matches ─ Groups"; tabs joined with " ─ ", active tab rendered green
	Update(msg tea.Msg, s *State) tea.Cmd
	View(s *State, w, h int) string
	Keys(s *State) []KeyHelp     // shown in bottom bar and help popup "Local" section
}
type KeyHelp struct{ Key, Desc string }
type State struct { // shared model state all panels read
	Flavor engine.Flavor; Flags map[engine.Flavor]string; Mode engine.Mode
	Pattern, Replacement, Text string; Manual, InvalidUTF8 bool
	Result *engine.Result; Err error; SelMatch, SelRow int; Focus PanelID; Tab map[PanelID]int
	Mouse bool; LeftWidth int
}
```

- **Layout:** the left column is 400 px in the mockup; in cells, use 34% of the width, clamped to 28–60 cells. It holds 1–4 stacked panels, with [3] taking the remaining height. The right column holds [5] (auto height), [6] (flexible) and [7] (40% of the height). Rounded borders, the focused panel's border green, the title drawn on the top border, and the "n of m" counter on the bottom border.
- **Global keys:** `1`–`7` focus a panel; `tab`/`shift-tab` cycle; `[`/`]` switch tab; `+`/`_` change LeftWidth by 4; `?` opens help (Task 15); `:` opens the prompt (Task 15); `q` quits. Inside an editor (panels 5 and 6), only `tab`, `shift-tab` and `esc` are global, and `esc` leaves the editor.

- [ ] **Step 1: Write the failing tests** using `teatest` with a fixed 160×48 size and a runner built on the fake engines from Task 8.
  - `TestShellRendersSevenPanels`: the output contains `[1]─Flavor` … `[7]─Explanation ─ Code ─ Compare`.
  - `TestFocusNumberKeys`: pressing `3` focuses Matches (the border colour token is in the output; assert through `State.Focus`, exposed by a test hook).
  - `TestTabCyclesFocus`.
  - `TestKeyBarFollowsFocus`: with [1] focused, the bar contains `Select flavor: enter`.
  - `TestResizeLeftColumn`.
  - `TestQuit`.
- [ ] **Step 2: Run** `go test ./internal/ui/`. Expected: FAIL.
- [ ] **Step 3: Implement** the shell with placeholder `View`s for the panels that Tasks 14–16 fill in. `theme.go` defines named colour tokens (`border`, `focus`, `title`, `key`, `dim`, `group1..6`, `match`, `error`, `ok`) using the mockup palette, with `lipgloss.AdaptiveColor` light variants.
- [ ] **Step 4: Run.** Expected: PASS.
- [ ] **Step 5: Commit** `feat(ui): lazygit-style shell, focus and key bar`.

---

### Task 14: TUI: editors, evaluation loop, flavor, flags and matches

**Files:**
- Create: `internal/ui/pattern.go`, `internal/ui/text.go`, `internal/ui/highlight.go`, `internal/ui/flavor.go`, `internal/ui/flags.go`, `internal/ui/matches.go`, `internal/ui/eval.go`, `internal/ui/editors_test.go`

**Interfaces:**
- Consumes: `runner.Eval/EvalAll/Events/Load/WarmUp`, `engine.ToByteOffsets`, `explain.Parse` (pattern syntax colouring), `hints.For`.
- Produces: `func highlightText(text string, res engine.Result, sel int) string` and `func highlightPattern(pattern string, f engine.Flavor, flags string, underline [2]int) string`. Task 16 reuses both.

Behaviour:
- **Evaluation:** a debounced eval runs 75 ms after the last edit through a `tea.Tick`. A stale result whose sequence number is older than the latest is dropped. In `Manual` mode, evaluation runs on `enter` instead, with the notice `large input: press enter to evaluate`. `InvalidUTF8` shows a `⚠ input had invalid UTF-8 (replaced with �)` line in [6].
- **Loading states:** while an engine loads, [5] shows `loading <flavor> engine…`; on `ErrTimeout`, it shows the error text in red. On startup, the default flavor loads, and once it reaches Ready the runner calls `WarmUp`.
- **[1] Flavor:** ↑/↓ or `j`/`k` move, and `enter` selects. Each row shows the engine name and version, plus a red `✗` marker. The markers come from `EvalAll` with a 300 ms debounce and only cover loaded engines.
- **[2] Flags:** lists the selected flavor's `Info().Flags`; `space` toggles. Flags are stored per flavor.
- **[5] Pattern:** tabs Match, Replace and Split set `Mode`. The Replace tab adds a second line, `replace`, and ↑/↓ move between the lines. The editor uses the bubbles textarea; undo is `ctrl-z` and redo is `ctrl-y`. On a compile error, the border turns red, the error message is shown verbatim under the pattern, and an underline marks the error's Offset when it is ≥ 0.
- **[6] Test String:** the match background colour, with each group in its own colour (group i uses token `group((i-1)%6+1)`). The selected match is outlined (reverse attribute on the line gutter). In Replace or Split mode, it splits into Test String | Result.
- **[3] Matches:** ▸ and ▾ expand a match into its groups (`enter`). `o` scrolls [6] to the match, `y` copies the value via OSC 52, and `Y` copies all matches as JSON. Spans are shown in native units, and a not-matched group shows `not matched`.

- [ ] **Step 1: Write the failing tests** (teatest with fake engines plus the real goengine).
  - `TestTypingEvaluatesDebounced`: typing `\d+` gives exactly one Eval after the 75 ms tick.
  - `TestSwitchFlavorKeepsPattern`: switching from Python to Go leaves the pattern string unchanged, and [5] shows the Go CompileError message verbatim.
  - `TestFlagsPerFlavor`.
  - `TestReplaceTabShowsResult`.
  - `TestUnmatchedGroupRendersNotMatched`.
  - `TestHighlightUTF16Offsets`: the result from a fake JS engine with UTF-16 spans highlights `😀` correctly (assert the ANSI-stripped segment boundaries).
  - `TestManualModeNeedsEnter`.
  - `TestInvalidUTF8Warning`.
  - `TestLoadingStateShown`.
  - `TestFlavorErrorMarker`.
- [ ] **Step 2: Run.** Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run.** Expected: PASS (`-race`).
- [ ] **Step 5: Commit** `feat(ui): editors, live evaluation, flavor/flags/matches panels`.

---

### Task 15: TUI: details panel, library, prompt and help

**Files:**
- Create: `internal/ui/details.go` (Explanation, Code and Compare tabs), `internal/ui/library.go`, `internal/ui/prompt.go`, `internal/ui/help.go`, `internal/ui/details_test.go`

**Interfaces:**
- Consumes: `explain.Explain`, `codegen.Snippet`, `hints.For`, `runner.EvalAll`, `config.Library`, `highlightPattern`.
- Produces: `func runCommand(line string, s *State) (tea.Cmd, error)`. Commands: `flavor <python|js|pcre|go>`, `flags <letters>`, `mouse [on|off]`, `save <name>`, `open <name>`, `mode <match|replace|split>`, `q`. Unknown commands give `unknown command: <x>`.

Behaviour:
- **[7] Explanation:** ↑/↓ select a row, and the selected row's Span is underlined in [5] (via `highlightPattern`). Unknown rows show in the dim colour.
- **[7] Code:** shows the snippet; `y` copies it.
- **[7] Compare:** runs `EvalAll` only while visible. It shows a row per flavor (✓ compiles + the first 3 matched values, or ✗ error) and, below, the Hint's `Why:`/`Try:` for the current flavor's failure. `a` applies the Rewrite to the pattern (the only path that edits the pattern), and `enter` switches to the row's flavor.
- **[4] Library:** `enter` loads an entry (flavor, flags, pattern, replacement, text); `n` saves the current state, asking for a name in the prompt; `d` deletes after a `y/n` prompt line, never a modal OS dialog. Saving is also available as `:save <name>`. Per Global Constraints there is no `ctrl-s`, and the help popup lists `n` (in Library) and `:save`.
- **Help (`?`):** a popup with `Local · <panel>` keys first, then Global, then Mouse, with a `/` filter that narrows rows by substring. `esc` closes it.
- **Prompt (`:`):** a one-line input at the bottom with tab-completion of command names and flavor names.

- [ ] **Step 1: Write the failing tests.**
  - `TestExplanationSelectUnderlines`: selecting the `(?P<level>…)` row underlines bytes [29,55) of the mockup pattern in [5].
  - `TestCompareApplyRewrite`: with flavor Go and pattern `(?<=\$)\d+(\.\d{2})?`, pressing `a` in Compare makes the pattern `\$(\d+(\.\d{2})?)`.
  - `TestCompareOnlyWhenVisible`: no EvalAll happens while Explanation is the active tab.
  - `TestCodeTabSnippet`.
  - `TestLibrarySaveOpen`.
  - `TestPromptCommands` (table).
  - `TestHelpFilter`.
  - `TestNoBindingOnForbiddenCtrlKeys`: walk the keymap and assert that none of `ctrl+m`, `ctrl+i`, `ctrl+[`, `ctrl+s`, `ctrl+q` is bound.
- [ ] **Step 2: Run.** Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run.** Expected: PASS.
- [ ] **Step 5: Commit** `feat(ui): explanation/code/compare, library, command prompt and help`.

---

### Task 16: TUI: mouse

**Files:**
- Create: `internal/ui/mouse.go`, `internal/ui/mouse_test.go`
- Modify: `internal/ui/app.go` (enable `tea.WithMouseCellMotion()` when `State.Mouse`; toggle at runtime)

**Interfaces:**
- Produces: `type hitMap struct{...}` rebuilt on each `View` from the layout rectangles, mapping a cell to (PanelID, row index | tab index | pattern byte offset); `func (h hitMap) At(x, y int) (Hit, bool)`.

Behaviour:
- Clicking a panel focuses it. Clicking a row selects it; in [1] it also switches flavor, and in [4] it loads the entry. Clicking a tab label switches tab.
- Clicking a pattern character selects its explanation row (the deepest row whose Span contains the offset).
- The wheel scrolls the panel under the cursor.
- Dragging the border between the columns sets LeftWidth.
- With the mouse off, no mouse mode is requested, so the terminal's native selection works.

- [ ] **Step 1: Write the failing tests** (sending `tea.MouseMsg`).
  - `TestClickFocusesPanel`.
  - `TestClickFlavorRowSwitches`.
  - `TestClickTabSwitches`.
  - `TestClickPatternSelectsExplanationRow`.
  - `TestWheelScrollsText`.
  - `TestDragResizes`.
  - `TestMouseOffIgnoresMouse` (`:mouse off`, then a click changes nothing).
- [ ] **Step 2: Run.** Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run.** Expected: PASS.
- [ ] **Step 5: Commit** `feat(ui): optional mouse support`.

---

### Task 17: Wire the real engines into the app

**Files:**
- Create: `internal/engine/all/all.go`, `internal/engine/all/all_test.go`
- Modify: `cmd/lazyregex/main.go`

**Interfaces:**
- Produces: `func Factories() map[engine.Flavor]runner.Factory`, which returns the Go, PCRE, JS and Python constructors.

- [ ] **Step 1: Write the failing test.** `TestAllEnginesAgreeOnSimpleCase`: `\d+` on `"a1b22"` with `g` gives the same two spans, {1,2} and {3,5}, in all four flavors (ASCII text, so the units coincide).
- [ ] **Step 2: Run.** Expected: FAIL.
- [ ] **Step 3: Implement `Factories`.** `main` builds `runner.New(all.Factories(), cacheDir, cfg.Timeout)` and `ui.New`, then runs `tea.NewProgram(model, tea.WithAltScreen())`.
- [ ] **Step 4: Run** `go test ./...`, then `make build && ./bin/lazyregex` by hand: the UI paints at once and the Python engine becomes Ready. Expected: PASS.
- [ ] **Step 5: Commit** `feat: wire bundled engines into the TUI`.

---

### Task 18: Command line

**Files:**
- Modify: `cmd/lazyregex/main.go`, `cmd/lazyregex/main_test.go`
- Create: `cmd/lazyregex/licenses.go`, `LICENSES/{CPython-PSF,PCRE2-BSD,QuickJS-ng-MIT,cJSON-MIT}.txt`

**Interfaces:**
- Consumes: `runner.PrepareText`, `explain.Explain/Render`, `config.Load`, `ui.New`.
- Produces the flags:
  - `-f, --flavor` (python|js|pcre|go)
  - `-p, --pattern`
  - `--explain` (prints `Render(Explain(pattern, flavor, flags))` and exits 0; the pattern comes from `-p` or the first positional argument)
  - `--flags`
  - `--version`
  - `--licenses` (prints the embedded LICENSES files, each under a header)

  Stdin that isn't a TTY becomes the test string through `PrepareText`, and the TUI then reopens `/dev/tty` (`CONIN$` on Windows) for keyboard input. An invalid `--flavor` value prints `unknown flavor "x" (want python, js, pcre, go)` and exits 2.

- [ ] **Step 1: Write the failing tests.**
  - `TestCLIExplain`: `--explain -f python '^\d+$'` prints a tree that contains `a digit, 1 or more`.
  - `TestCLIUnknownFlavor`: exit code 2.
  - `TestCLILicenses`: the output contains `PCRE2` and `Python Software Foundation`.
  - `TestCLIStdinBinary`: `run` with 2,000,000 random bytes on stdin and a test hook that captures `ui.Deps` instead of starting the program gives `TextPrepared.Manual` and `InvalidUTF8` both true.
  - `TestCLIPresetFlavorPattern`.
- [ ] **Step 2: Run.** Expected: FAIL.
- [ ] **Step 3: Implement.** Embed the licences with `//go:embed`.
- [ ] **Step 4: Run.** Expected: PASS.
- [ ] **Step 5: Commit** `feat(cli): flags, stdin input, --explain and --licenses`.

---

### Task 19: Release pipeline and README

**Files:**
- Create: `.goreleaser.yaml`, `.github/workflows/release.yaml`, `docs/RELEASING.md`
- Modify: `README.md`, `Makefile` (`release-snapshot: goreleaser release --snapshot --clean`)

**Interfaces:**
- Consumes: the `version.Version` ldflag.
- Produces the release artifacts.

**`.goreleaser.yaml`:**
- `builds`: `CGO_ENABLED=0`, the six platforms, and ldflags setting the version.
- `archives`: tar.gz (zip on Windows), including `LICENSE` and `LICENSES/**`.
- `checksum`.
- `nfpms`: deb and rpm.
- `brews`: repo `dor1202/homebrew-lazyregex`, token `{{ .Env.HOMEBREW_TAP_GITHUB_TOKEN }}`, `test: system "#{bin}/lazyregex --version"`. This replaces the Python formula.
- `scoops`: repo `dor1202/scoop-lazyregex`, `skip_upload: '{{ if not (index .Env "SCOOP_GITHUB_TOKEN") }}true{{ end }}'`.
- `aurs`: `lazyregex-bin`, skipped the same way when `AUR_KEY` is unset.
- `chocolateys`: skipped when `CHOCOLATEY_API_KEY` is unset (it runs only in the Windows job).

**`release.yaml`:** on `v*` tags, runs goreleaser on ubuntu with all secrets passed through, plus a separate windows-latest job that runs only the chocolatey publisher when the key exists.

**`docs/RELEASING.md`:** lists each secret, who creates it, and what's skipped without it: `HOMEBREW_TAP_GITHUB_TOKEN`, `SCOOP_GITHUB_TOKEN` plus the bucket repo, `AUR_KEY` plus the AUR account, `CHOCOLATEY_API_KEY`. v1.0.0 needs only the first one.

**README:** what it is; a GIF placeholder `assets/demo.gif` (recorded by hand after the build); install for brew, scoop, choco, deb/rpm, AUR and `go install`; a flavor table (engine, pinned version, span unit); the known QuickJS vs V8 differences (from `parity/exceptions.yaml`); keybindings; a CLI section; and a note that v0.x was the Python version.

- [ ] **Step 1: Write `.goreleaser.yaml`** and run `goreleaser check`. Expected: no errors.
- [ ] **Step 2: Run `make release-snapshot`.** Expected: six archives, a deb, an rpm, and a generated brew formula in `dist/`. `dist/lazyregex_darwin_arm64*/lazyregex --version` prints a snapshot version. Record the archive sizes; expect about 30–40 MB.
- [ ] **Step 3: Write `release.yaml`, `RELEASING.md` and the README.**
- [ ] **Step 4: Check the size budget.** `go test ./...` passes, and `ls -la dist/*.tar.gz` shows each archive under 60 MB. If one is over, trim `stdlib-allowlist.txt`.
- [ ] **Step 5: Commit** `build: GoReleaser pipeline, release docs and README`.

---

## Self-review notes

- **Spec coverage:** every section of the spec maps to at least one task.
  - Engines, exactness and spans: Tasks 2 and 4–7.
  - Lazy start, warm-up, timeouts and text input: Task 8.
  - Explanations: Tasks 9–10.
  - The panels: Tasks 13–16.
  - Code tab and hints: Task 11.
  - Library and config: Task 12.
  - CLI: Task 18.
  - Releasing and gating: Task 19.
  - Parity testing: Task 3, with each engine adding its flavor.
  - Migration: Tasks 1 and 19.
  - Licences: Tasks 18 and 19.
- **Deliberate deviation from the spec:** the spec lists `ctrl-s` for saving. It's dropped (flow-control collision) in favour of `n` in Library and `:save`, as the spec asked the builder to check those collisions.
