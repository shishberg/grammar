package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(dir, name, content string) error {
	return os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
}

func TestRun_HappyPath_SingleFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"-dir", "testdata/single", "-rule", "greeting", "-seed", "1"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit=%d stderr=%q", code, stderr.String())
	}
	got := strings.TrimRight(stdout.String(), "\n")
	if got != "hello" {
		t.Fatalf("stdout = %q, want %q", got, "hello")
	}
}

func TestRun_NProducesNLines(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"-dir", "testdata/single", "-rule", "greeting", "-n", "4", "-seed", "1"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit=%d stderr=%q", code, stderr.String())
	}
	lines := strings.Split(strings.TrimRight(stdout.String(), "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("got %d lines, want 4: %q", len(lines), stdout.String())
	}
	for i, l := range lines {
		if l != "hello" {
			t.Errorf("line %d = %q, want %q", i, l, "hello")
		}
	}
}

func TestRun_SeedDeterministic(t *testing.T) {
	args := []string{"-dir", "testdata/seeded", "-rule", "word", "-n", "5", "-seed", "42"}
	var first bytes.Buffer
	if code := run(args, &first, &bytes.Buffer{}); code != 0 {
		t.Fatalf("run exit=%d", code)
	}
	var second bytes.Buffer
	if code := run(args, &second, &bytes.Buffer{}); code != 0 {
		t.Fatalf("run exit=%d", code)
	}
	if first.String() != second.String() {
		t.Fatalf("seed=42 not deterministic:\nfirst:  %q\nsecond: %q", first.String(), second.String())
	}
	lines := strings.Split(strings.TrimRight(first.String(), "\n"), "\n")
	if len(lines) != 5 {
		t.Fatalf("got %d lines, want 5", len(lines))
	}
	// At least two distinct outputs to confirm the rule actually varies.
	distinct := map[string]struct{}{}
	for _, l := range lines {
		distinct[l] = struct{}{}
	}
	if len(distinct) < 2 {
		t.Fatalf("only %d distinct lines from 5 generations: %q", len(distinct), first.String())
	}
}

func TestRun_PostProcChain(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"-dir", "testdata/postproc", "-rule", "thing", "-postproc", "underscore,aan", "-seed", "1"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit=%d stderr=%q", code, stderr.String())
	}
	got := strings.TrimRight(stdout.String(), "\n")
	if got != "a fire axe" {
		t.Fatalf("stdout = %q, want %q", got, "a fire axe")
	}
}

func TestRun_PostProcAanRewritesArticle(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"-dir", "testdata/postproc", "-rule", "vowel", "-postproc", "underscore,aan", "-seed", "1"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit=%d stderr=%q", code, stderr.String())
	}
	got := strings.TrimRight(stdout.String(), "\n")
	if got != "an iron axe" {
		t.Fatalf("stdout = %q, want %q", got, "an iron axe")
	}
}

func TestRun_HelpExitsZero(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"-h"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "-dir") {
		t.Errorf("help output should mention -dir: %q", stderr.String())
	}
}

func TestRun_MissingDir(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"-rule", "greeting"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit=%d, want 2; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "-dir") {
		t.Errorf("stderr missing -dir mention: %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "Usage") {
		t.Errorf("stderr missing usage: %q", stderr.String())
	}
}

func TestRun_MissingRule(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"-dir", "testdata/single"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit=%d, want 2; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "-rule") {
		t.Errorf("stderr missing -rule mention: %q", stderr.String())
	}
}

func TestRun_BadDir(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"-dir", "testdata/does-not-exist", "-rule", "x"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit=%d, want 1; stderr=%q", code, stderr.String())
	}
	if stderr.Len() == 0 {
		t.Errorf("expected an error message on stderr")
	}
}

func TestRun_DirIsFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"-dir", "testdata/single/greet.grammar", "-rule", "x"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit=%d, want 1; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "not a directory") {
		t.Errorf("stderr should explain dir-vs-file: %q", stderr.String())
	}
}

func TestRun_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := run([]string{"-dir", dir, "-rule", "x"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit=%d, want 1; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "no .grammar files") {
		t.Errorf("stderr should explain empty dir: %q", stderr.String())
	}
}

func TestRun_UnknownPostProc(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"-dir", "testdata/single", "-rule", "greeting", "-postproc", "bogus"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit=%d, want 1; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "bogus") {
		t.Errorf("stderr should name the bad processor: %q", stderr.String())
	}
}

func TestRun_ParseErrorReportsFilename(t *testing.T) {
	dir := t.TempDir()
	// Two files; the bad one must come second alphabetically so we
	// know we're not just reporting "the first file we tried".
	good := "rule a\n  forms: default\n  one\n"
	bad := "this is not a valid grammar file\n"
	if err := writeFile(dir, "a_good.grammar", good); err != nil {
		t.Fatal(err)
	}
	if err := writeFile(dir, "b_bad.grammar", bad); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run([]string{"-dir", dir, "-rule", "a"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit=%d, want 1; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "b_bad.grammar") {
		t.Errorf("stderr should name the failing file: %q", stderr.String())
	}
}

// Cross-file references: parts file defines a rule that the top file
// references. Each file Parses on its own; Validate after Merge passes.
func TestRun_CrossFileReferenceResolves(t *testing.T) {
	dir := t.TempDir()
	parts := "rule color\n  red\n"
	top := "rule sentence\n  the {color} thing\n"
	if err := writeFile(dir, "a_parts.grammar", parts); err != nil {
		t.Fatal(err)
	}
	if err := writeFile(dir, "b_top.grammar", top); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run([]string{"-dir", dir, "-rule", "sentence", "-seed", "1"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit=%d stderr=%q", code, stderr.String())
	}
	got := strings.TrimRight(stdout.String(), "\n")
	if got != "the red thing" {
		t.Fatalf("stdout = %q, want %q", got, "the red thing")
	}
}

// Cross-file failure: a typo'd reference parses fine on its own but
// fails the post-merge Validate, with a non-zero exit.
func TestRun_CrossFileTypoFailsValidate(t *testing.T) {
	dir := t.TempDir()
	parts := "rule color\n  red\n"
	top := "rule sentence\n  the {colour} thing\n"
	if err := writeFile(dir, "a_parts.grammar", parts); err != nil {
		t.Fatal(err)
	}
	if err := writeFile(dir, "b_top.grammar", top); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run([]string{"-dir", dir, "-rule", "sentence", "-seed", "1"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit=%d, want 1; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "colour") {
		t.Errorf("stderr should name the missing rule: %q", stderr.String())
	}
}

func TestRun_NonPositiveNRejected(t *testing.T) {
	for _, n := range []string{"0", "-3"} {
		t.Run("n="+n, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run([]string{"-dir", "testdata/single", "-rule", "greeting", "-n", n}, &stdout, &stderr)
			if code != 2 {
				t.Fatalf("exit=%d, want 2; stderr=%q", code, stderr.String())
			}
			if !strings.Contains(stderr.String(), "-n") {
				t.Errorf("stderr should mention -n: %q", stderr.String())
			}
			if stdout.Len() != 0 {
				t.Errorf("stdout should be empty: %q", stdout.String())
			}
		})
	}
}

func TestRun_MergesMultipleFiles(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"-dir", "testdata/multi", "-rule", "sentence", "-seed", "1"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit=%d stderr=%q", code, stderr.String())
	}
	got := strings.TrimRight(stdout.String(), "\n")
	if got != "the red thing" {
		t.Fatalf("stdout = %q, want %q", got, "the red thing")
	}
}
