// grammar reads a directory of .grammar files, merges them, and prints
// generations from a named rule.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/shishberg/grammar"
	"github.com/shishberg/grammar/postproc/english"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("grammar", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("dir", "", "directory of .grammar files (required)")
	rule := fs.String("rule", "", "rule name to generate (required)")
	n := fs.Int("n", 1, "number of generations to print")
	seed := fs.Int64("seed", 0, "RNG seed; 0 means use current time")
	postproc := fs.String("postproc", "", "comma-separated post-processors: aan, underscore")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if *dir == "" || *rule == "" {
		fmt.Fprintln(stderr, "grammar: -dir and -rule are required")
		fs.Usage()
		return 2
	}
	if *n < 1 {
		fmt.Fprintln(stderr, "grammar: -n must be at least 1")
		return 2
	}

	info, err := os.Stat(*dir)
	if err != nil {
		fmt.Fprintf(stderr, "grammar: %v\n", err)
		return 1
	}
	if !info.IsDir() {
		fmt.Fprintf(stderr, "grammar: %s is not a directory\n", *dir)
		return 1
	}

	entries, err := os.ReadDir(*dir)
	if err != nil {
		fmt.Fprintf(stderr, "grammar: %v\n", err)
		return 1
	}
	// os.ReadDir returns entries sorted by name, so iterating `entries`
	// directly yields a deterministic concatenation order.
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(e.Name(), ".grammar") {
			continue
		}
		files = append(files, e.Name())
	}
	if len(files) == 0 {
		fmt.Fprintf(stderr, "grammar: no .grammar files in %s\n", *dir)
		return 1
	}

	// Parse each file on its own and merge the results. Per-file Parse
	// only checks per-file syntax, so a parse error attributes cleanly
	// to the file it came from. Cross-file reference resolution is
	// deferred to Validate, which runs once after the merge.
	g := grammar.NewGrammar()
	for _, name := range files {
		path := filepath.Join(*dir, name)
		src, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", name, err)
			return 1
		}
		gFile, err := grammar.Parse(string(src))
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", name, err)
			return 1
		}
		if err := g.Merge(gFile); err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", name, err)
			return 1
		}
	}
	if err := g.Validate(); err != nil {
		fmt.Fprintf(stderr, "grammar: %v\n", err)
		return 1
	}

	procs, err := resolvePostProcs(*postproc)
	if err != nil {
		fmt.Fprintf(stderr, "grammar: %v\n", err)
		return 1
	}

	s := *seed
	if s == 0 {
		s = time.Now().UnixNano()
	}
	rng := rand.New(rand.NewSource(s))

	for i := 0; i < *n; i++ {
		out, err := g.GenerateWith(*rule, rng, procs...)
		if err != nil {
			fmt.Fprintf(stderr, "grammar: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, out)
	}
	return 0
}

func resolvePostProcs(spec string) ([]grammar.PostProcessor, error) {
	if spec == "" {
		return nil, nil
	}
	var out []grammar.PostProcessor
	for name := range strings.SplitSeq(spec, ",") {
		switch strings.TrimSpace(name) {
		case "aan":
			out = append(out, english.AAn)
		case "underscore":
			out = append(out, english.UnderscoreToSpace)
		default:
			return nil, fmt.Errorf("unknown post-processor %q", name)
		}
	}
	return out, nil
}
