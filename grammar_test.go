package grammar

import (
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
