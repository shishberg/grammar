package grammar

import (
	"strings"
	"testing"
)

func TestGenerateWithAppliesPostProcessorsInOrder(t *testing.T) {
	g := singleAlt("r", "abc")
	upper := func(s string) string { return strings.ToUpper(s) }
	suffix := func(s string) string { return s + "-X" }
	out, err := g.GenerateWith("r", newRand(1), upper, suffix)
	if err != nil {
		t.Fatalf("GenerateWith: %v", err)
	}
	if out != "ABC-X" {
		t.Errorf("out = %q, want %q", out, "ABC-X")
	}
}

func TestGenerateWithNoProcessorsMatchesGenerate(t *testing.T) {
	g := singleAlt("r", "hello")
	out, err := g.GenerateWith("r", newRand(1))
	if err != nil {
		t.Fatalf("GenerateWith: %v", err)
	}
	if out != "hello" {
		t.Errorf("out = %q, want %q", out, "hello")
	}
}
