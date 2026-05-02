package grammar

import (
	"strings"
	"testing"
)

// TestSmokeProgrammaticConstruction asserts the conceptual-model types
// can be wired together without methods. It's a compile-and-shape check;
// behavior coverage lives in TestGenerate*.
func TestSmokeProgrammaticConstruction(t *testing.T) {
	g := &Grammar{
		rules: map[string]*Rule{
			"greeting": {
				Forms: []FormSpec{{Name: "default"}},
				Alternatives: []Alternative{
					{Weight: 1, Forms: map[string]Template{
						"default": {Literal{Text: "hello"}},
					}},
				},
			},
		},
	}
	if g.rules["greeting"] == nil {
		t.Fatal("rule not stored")
	}
	if len(g.rules["greeting"].Alternatives) != 1 {
		t.Fatalf("alternatives = %d, want 1", len(g.rules["greeting"].Alternatives))
	}
	// Token interface seal: a heterogeneous slice should compile.
	var _ Template = Template{
		Literal{Text: "x"},
		RuleRef{Rule: "y"},
		Recall{Name: "Z"},
		SelfRef{},
	}
}

func TestAddRuleRejectsInvalidAlternativeTag(t *testing.T) {
	for _, tag := range []string{"fruit=red", "bad\x00tag"} {
		g := NewGrammar()
		err := g.AddRule("snack", &Rule{
			Forms: []FormSpec{{Name: "default"}},
			Alternatives: []Alternative{
				{Tags: []string{tag}, Forms: map[string]Template{"default": {Literal{Text: "apple"}}}},
			},
		})
		if err == nil {
			t.Fatalf("AddRule with tag %q returned nil error", tag)
		}
		if !strings.Contains(err.Error(), "invalid tag") {
			t.Fatalf("err = %v, want invalid tag", err)
		}
	}
}

func TestAddRuleAllowsDelimiterFreeAlternativeTags(t *testing.T) {
	g := NewGrammar()
	err := g.AddRule("snack", &Rule{
		Forms: []FormSpec{{Name: "default"}},
		Alternatives: []Alternative{
			{Tags: []string{"fruit-🍎", "mood/happy"}, Forms: map[string]Template{"default": {Literal{Text: "apple"}}}},
		},
	})
	if err != nil {
		t.Fatalf("AddRule: %v", err)
	}
}

func TestAddRuleValidatesRuleRefTags(t *testing.T) {
	g := NewGrammar()
	err := g.AddRule("snack", &Rule{
		Forms: []FormSpec{{Name: "default"}},
		Alternatives: []Alternative{
			{Forms: map[string]Template{"default": {RuleRef{Rule: "filling", Tags: []string{"fruit-🍎"}, Required: []string{"mood/happy"}}}}},
		},
	})
	if err != nil {
		t.Fatalf("AddRule: %v", err)
	}

	err = g.AddRule("meal", &Rule{
		Forms: []FormSpec{{Name: "default"}},
		Alternatives: []Alternative{
			{Forms: map[string]Template{"default": {RuleRef{Rule: "snack", Required: []string{"bad=tag"}}}}},
		},
	})
	if err == nil {
		t.Fatal("AddRule returned nil error")
	}
	if !strings.Contains(err.Error(), "invalid tag") {
		t.Fatalf("err = %v, want invalid tag", err)
	}
}
