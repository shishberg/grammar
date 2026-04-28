package english_test

import (
	"math/rand"
	"testing"

	"github.com/shishberg/grammar"
	"github.com/shishberg/grammar/postproc/english"
)

// TestIntegrationWithGenerateWith confirms the post-processors satisfy
// grammar.PostProcessor and run in the order callers pass them.
func TestIntegrationWithGenerateWith(t *testing.T) {
	src := `rule sentence
  forms: default
  a apple_pie
`
	g, err := grammar.Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	rng := rand.New(rand.NewSource(1))
	out, err := g.GenerateWith("sentence", rng, english.UnderscoreToSpace, english.AAn)
	if err != nil {
		t.Fatalf("GenerateWith: %v", err)
	}
	if out != "an apple pie" {
		t.Errorf("out = %q, want %q", out, "an apple pie")
	}
}
