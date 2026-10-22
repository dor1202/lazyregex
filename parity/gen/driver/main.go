// Command driver generates parity/expected/<flavor>/<corpus>.json by running
// the corpus through the flavor's native runtime generator (or goengine
// in-process for the go flavor).
//
//	go run ./parity/gen/driver -flavor python -out parity/expected
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dor1202/lazyregex/internal/engine"
	"github.com/dor1202/lazyregex/internal/engine/goengine"
	"github.com/dor1202/lazyregex/parity"
)

var generators = map[engine.Flavor]string{
	engine.Python: "python3 {dir}/gen/gen_python.py",
	engine.JS:     "node {dir}/gen/gen_node.mjs",
	engine.PCRE:   "php {dir}/gen/gen_php.php",
}

func main() {
	flavor := flag.String("flavor", "", "python, js, pcre or go")
	out := flag.String("out", "parity/expected", "output directory (a <flavor> subdirectory is written)")
	cmdOverride := flag.String("cmd", "", "override the generator command (space separated); it reads cases as JSON on stdin")
	flag.Parse()
	if err := run(engine.Flavor(*flavor), *out, *cmdOverride); err != nil {
		fmt.Fprintln(os.Stderr, "driver:", err)
		os.Exit(1)
	}
}

func run(flavor engine.Flavor, out, cmdOverride string) error {
	dir := parity.Dir()
	corpus, err := parity.LoadCorpus(filepath.Join(dir, "corpus"))
	if err != nil {
		return err
	}
	known := false
	for _, f := range engine.Flavors {
		known = known || f == flavor
	}
	if !known {
		return fmt.Errorf("unknown or missing -flavor %q", flavor)
	}
	names := make([]string, 0, len(corpus))
	for n := range corpus {
		names = append(names, n)
	}
	sort.Strings(names)

	flavorDir := filepath.Join(out, string(flavor))
	if err := os.MkdirAll(flavorDir, 0o755); err != nil {
		return err
	}
	for _, name := range names {
		var cases []parity.Case
		for _, c := range corpus[name] {
			if c.AppliesTo(flavor) {
				cases = append(cases, c)
			}
		}
		if len(cases) == 0 {
			continue
		}
		var results []parity.Expected
		if flavor == engine.Go {
			results, err = runGo(cases)
		} else {
			results, err = runExternal(flavor, dir, cmdOverride, cases)
		}
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if len(results) != len(cases) {
			return fmt.Errorf("%s: generator returned %d results for %d cases", name, len(results), len(cases))
		}
		for i := range results {
			if results[i].ID != cases[i].ID {
				return fmt.Errorf("%s: result %d has id %q, want %q", name, i, results[i].ID, cases[i].ID)
			}
			results[i].Normalize()
		}
		b, err := marshal(results)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(flavorDir, name+".json"), b, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func marshal(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func runGo(cases []parity.Case) ([]parity.Expected, error) {
	e := goengine.New()
	var out []parity.Expected
	for _, c := range cases {
		x, err := parity.Evaluate(context.Background(), e, c)
		if err != nil {
			return nil, fmt.Errorf("case %s: %w", c.ID, err)
		}
		out = append(out, x)
	}
	return out, nil
}

func runExternal(flavor engine.Flavor, dir, override string, cases []parity.Case) ([]parity.Expected, error) {
	spec := override
	if spec == "" {
		spec = strings.ReplaceAll(generators[flavor], "{dir}", dir)
	}
	argv := strings.Fields(spec)
	in, err := json.Marshal(cases)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdin = bytes.NewReader(in)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%s: %w", argv[0], err)
	}
	var out []parity.Expected
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		return nil, fmt.Errorf("decoding generator output: %w", err)
	}
	return out, nil
}
