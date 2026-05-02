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

func TestGenerateWithOptionsCombinesTagsAndPostProcessors(t *testing.T) {
	g := &Grammar{rules: map[string]*Rule{
		"r": {
			Forms: []FormSpec{{Name: "default"}},
			Alternatives: []Alternative{
				{Tags: []string{"fruit"}, Forms: map[string]Template{"default": {Literal{Text: "apple"}}}},
			},
		},
	}}
	out, err := g.GenerateWithOptions("r", newRand(1), []GenerateOption{WithTags("fruit")}, strings.ToUpper)
	if err != nil {
		t.Fatalf("GenerateWithOptions: %v", err)
	}
	if out != "APPLE" {
		t.Errorf("out = %q, want APPLE", out)
	}
}
