package grammar

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestMergeNonOverlapping(t *testing.T) {
	g := singleAlt("a", "alpha")
	other := singleAlt("b", "beta")
	if err := g.Merge(other); err != nil {
		t.Fatalf("Merge: %v", err)
	}
	out, err := g.Generate("a", newRand(1))
	if err != nil || out != "alpha" {
		t.Errorf("a: got (%q, %v)", out, err)
	}
	out, err = g.Generate("b", newRand(1))
	if err != nil || out != "beta" {
		t.Errorf("b: got (%q, %v)", out, err)
	}
}

// Two grammars define the same rule with the same form scheme: Merge
// appends other's alternatives onto g's. Weights are preserved.
func TestMergeSameRuleSameSchemeCombines(t *testing.T) {
	src1 := "rule color\n  red\n  weight=3 blue\n"
	src2 := "rule color\n  green\n  weight=2 yellow\n"
	g1, err := Parse(src1)
	if err != nil {
		t.Fatalf("Parse(src1): %v", err)
	}
	g2, err := Parse(src2)
	if err != nil {
		t.Fatalf("Parse(src2): %v", err)
	}
	if err := g1.Merge(g2); err != nil {
		t.Fatalf("Merge: %v", err)
	}
	r := g1.rules["color"]
	if len(r.Alternatives) != 4 {
		t.Fatalf("alternatives = %d, want 4", len(r.Alternatives))
	}
	wantText := []string{"red", "blue", "green", "yellow"}
	wantWeight := []uint{1, 3, 1, 2}
	for i := range wantText {
		lit, ok := r.Alternatives[i].Forms["default"][0].(Literal)
		if !ok || lit.Text != wantText[i] {
			t.Errorf("alt %d text = %#v, want %q", i, r.Alternatives[i].Forms["default"][0], wantText[i])
		}
		if r.Alternatives[i].Weight != wantWeight[i] {
			t.Errorf("alt %d weight = %d, want %d", i, r.Alternatives[i].Weight, wantWeight[i])
		}
	}
	// All four colours should be reachable from generation.
	got := map[string]bool{}
	for i := range 100 {
		out, err := g1.Generate("color", newRand(int64(i)))
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		got[out] = true
	}
	for _, w := range wantText {
		if !got[w] {
			t.Errorf("100 generations never produced %q (got %v)", w, got)
		}
	}
}

// Multi-form rules combine when the form names, order, and form-default
// templates all match.
func TestMergeSameRuleMatchingMultiFormScheme(t *testing.T) {
	src1 := "rule animal\n  forms: default, plural={}s\n  cat\n"
	src2 := "rule animal\n  forms: default, plural={}s\n  mouse | mice\n"
	g1, _ := Parse(src1)
	g2, _ := Parse(src2)
	if err := g1.Merge(g2); err != nil {
		t.Fatalf("Merge: %v", err)
	}
	r := g1.rules["animal"]
	if len(r.Alternatives) != 2 {
		t.Fatalf("alternatives = %d, want 2", len(r.Alternatives))
	}
}

// Form scheme mismatch: same rule name but different form names.
func TestMergeSameRuleDifferentFormNamesErrors(t *testing.T) {
	src1 := "rule animal\n  forms: default, plural={}s\n  cat\n"
	src2 := "rule animal\n  forms: default, past={}ed\n  bark\n"
	g1, _ := Parse(src1)
	g2, _ := Parse(src2)
	err := g1.Merge(g2)
	if !errors.Is(err, ErrFormSchemeMismatch) {
		t.Fatalf("err = %v; want errors.Is ErrFormSchemeMismatch", err)
	}
}

// Form scheme mismatch: same form names, different order.
func TestMergeSameRuleDifferentFormOrderErrors(t *testing.T) {
	src1 := "rule x\n  forms: default, plural={}s, past={}ed\n  a\n"
	src2 := "rule x\n  forms: default, past={}ed, plural={}s\n  b\n"
	g1, _ := Parse(src1)
	g2, _ := Parse(src2)
	err := g1.Merge(g2)
	if !errors.Is(err, ErrFormSchemeMismatch) {
		t.Fatalf("err = %v; want errors.Is ErrFormSchemeMismatch", err)
	}
}

// Form scheme mismatch: same form names but a non-default form's
// default template differs structurally.
func TestMergeSameRuleDifferentFormDefaultTemplateErrors(t *testing.T) {
	src1 := "rule x\n  forms: default, plural={}s\n  a\n"
	src2 := "rule x\n  forms: default, plural={}es\n  a\n"
	g1, _ := Parse(src1)
	g2, _ := Parse(src2)
	err := g1.Merge(g2)
	if !errors.Is(err, ErrFormSchemeMismatch) {
		t.Fatalf("err = %v; want errors.Is ErrFormSchemeMismatch", err)
	}
	if !strings.Contains(err.Error(), "x") {
		t.Errorf("error %q should name the rule", err)
	}
}

// Programmatic AddRule still uses ErrDuplicateRule; the Merge-combine
// behaviour does not extend to AddRule (there's no second-call to
// combine with — each AddRule installs one rule definition).
func TestAddRuleDuplicateStillErrors(t *testing.T) {
	g := singleAlt("a", "first")
	err := g.AddRule("a", &Rule{
		Forms:        []FormSpec{{Name: "default"}},
		Alternatives: []Alternative{{Forms: map[string]Template{"default": {Literal{Text: "second"}}}}},
	})
	if !errors.Is(err, ErrDuplicateRule) {
		t.Fatalf("err = %v; want errors.Is ErrDuplicateRule", err)
	}
}

// Sanity: reflect.DeepEqual is fine for template equality, including
// nil-vs-empty-slice equivalence we depend on (a default form's
// Default is nil; both sides of a same-scheme compare must agree).
func TestMergeFormDefaultTemplateNilEquality(t *testing.T) {
	g1 := singleAlt("a", "x")
	g2 := singleAlt("a", "y")
	// Both forms have nil Default for the default form.
	if !reflect.DeepEqual(g1.rules["a"].Forms[0].Default, g2.rules["a"].Forms[0].Default) {
		t.Fatal("default-form Default templates should be deep-equal")
	}
	if err := g1.Merge(g2); err != nil {
		t.Fatalf("Merge: %v", err)
	}
}

func TestMergeIntoEmpty(t *testing.T) {
	g := &Grammar{}
	other := singleAlt("a", "alpha")
	if err := g.Merge(other); err != nil {
		t.Fatalf("Merge: %v", err)
	}
	out, err := g.Generate("a", newRand(1))
	if err != nil || out != "alpha" {
		t.Errorf("got (%q, %v)", out, err)
	}
}

// Merge must not retain references to other's *Rule values. Mutating
// other after Merge — directly or via a second Merge — would
// otherwise silently corrupt g.
func TestMergeIsolatesRulesFromOther(t *testing.T) {
	g := NewGrammar()
	other := singleAlt("a", "first")
	if err := g.Merge(other); err != nil {
		t.Fatalf("Merge: %v", err)
	}
	// Append to other's rule's alternatives directly; g must not see
	// the new alternative.
	other.rules["a"].Alternatives = append(other.rules["a"].Alternatives, Alternative{
		Forms: map[string]Template{"default": {Literal{Text: "leaked"}}},
	})
	if got := len(g.rules["a"].Alternatives); got != 1 {
		t.Errorf("g's rule alternative count = %d, want 1 — Merge leaked other's slice", got)
	}
}

// Programmatic callers may build a FormSpec with Default left nil
// (matching what Parse produces) or as an explicit empty Template{}.
// Both encode "no default template" and must compare equal in form
// schemes during Merge.
func TestMergeNilAndEmptyTemplateAreEqual(t *testing.T) {
	mk := func(def Template) *Grammar {
		g := NewGrammar()
		_ = g.AddRule("x", &Rule{
			Forms: []FormSpec{
				{Name: "default"},
				{Name: "plural", Default: def},
			},
			Alternatives: []Alternative{{
				Forms: map[string]Template{"default": {Literal{Text: "a"}}},
			}},
		})
		return g
	}
	gNil := mk(nil)
	gEmpty := mk(Template{})
	// Build a third matching grammar to merge into; both should be
	// accepted regardless of nil-vs-empty.
	target := mk(nil)
	if err := target.Merge(gNil); err != nil {
		t.Fatalf("Merge nil: %v", err)
	}
	if err := target.Merge(gEmpty); err != nil {
		t.Fatalf("Merge empty: %v", err)
	}
}

// Merge(nil) is a documented no-op. The host grammar must keep its
// existing rules and remain usable.
func TestMergeNilIsNoOp(t *testing.T) {
	g := singleAlt("a", "alpha")
	if err := g.Merge(nil); err != nil {
		t.Fatalf("Merge(nil): %v", err)
	}
	out, err := g.Generate("a", newRand(1))
	if err != nil || out != "alpha" {
		t.Errorf("after Merge(nil): got (%q, %v)", out, err)
	}
}
