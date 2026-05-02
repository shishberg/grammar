package grammar

import (
	"errors"
	"math/rand"
	"strings"
	"testing"
)

func newRand(seed int64) *rand.Rand {
	return rand.New(rand.NewSource(seed))
}

// singleAlt builds a one-rule grammar whose default form expands to the
// given literal. It exclusively goes through the public NewGrammar /
// AddRule API, doubling as a worked example for external callers.
func singleAlt(rule, text string) *Grammar {
	g := NewGrammar()
	if err := g.AddRule(rule, &Rule{
		Forms: []FormSpec{{Name: "default"}},
		Alternatives: []Alternative{
			{Weight: 1, Forms: map[string]Template{
				"default": {Literal{Text: text}},
			}},
		},
	}); err != nil {
		panic(err)
	}
	return g
}

func TestGenerateSingleAlternative(t *testing.T) {
	g := singleAlt("greeting", "hello")
	out, err := g.Generate("greeting", newRand(1))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if out != "hello" {
		t.Errorf("out = %q, want %q", out, "hello")
	}
}

func TestGenerateUnknownRuleErrors(t *testing.T) {
	g := singleAlt("greeting", "hello")
	_, err := g.Generate("missing", newRand(1))
	if err == nil {
		t.Fatal("Generate(missing) returned nil error")
	}
	if !strings.Contains(err.Error(), "missing") {
		t.Errorf("error %q should name the missing rule", err)
	}
}

// TestGenerateWeightedPicks asserts that weights influence the
// distribution. We pick three alternatives with weights 1, 2, 3 and
// require all three to appear across many samples (probabilistic, but
// the seed is fixed so the run is deterministic).
func TestGenerateWeightedPicks(t *testing.T) {
	g := &Grammar{rules: map[string]*Rule{
		"choice": {
			Forms: []FormSpec{{Name: "default"}},
			Alternatives: []Alternative{
				{Weight: 1, Forms: map[string]Template{"default": {Literal{Text: "a"}}}},
				{Weight: 2, Forms: map[string]Template{"default": {Literal{Text: "b"}}}},
				{Weight: 3, Forms: map[string]Template{"default": {Literal{Text: "c"}}}},
			},
		},
	}}
	rng := newRand(42)
	counts := map[string]int{}
	const N = 600
	for range N {
		out, err := g.Generate("choice", rng)
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		counts[out]++
	}
	if counts["a"] == 0 || counts["b"] == 0 || counts["c"] == 0 {
		t.Fatalf("missing outcomes: %v", counts)
	}
	// b should clearly outnumber a; c should clearly outnumber b.
	if !(counts["a"] < counts["b"] && counts["b"] < counts["c"]) {
		t.Errorf("ordering violated, counts = %v", counts)
	}
}

// TestGenerateWeightedDeterministic pins the exact pick for a fixed seed
// so a regression in the random-pick algorithm shows up immediately.
func TestGenerateWeightedDeterministic(t *testing.T) {
	g := &Grammar{rules: map[string]*Rule{
		"choice": {
			Forms: []FormSpec{{Name: "default"}},
			Alternatives: []Alternative{
				{Weight: 1, Forms: map[string]Template{"default": {Literal{Text: "a"}}}},
				{Weight: 1, Forms: map[string]Template{"default": {Literal{Text: "b"}}}},
				{Weight: 1, Forms: map[string]Template{"default": {Literal{Text: "c"}}}},
			},
		},
	}}
	// Same seed, same sequence — record the first ten picks here so
	// future changes to the algorithm have to update this consciously.
	rng := newRand(7)
	got := make([]string, 0, 10)
	for range 10 {
		s, err := g.Generate("choice", rng)
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		got = append(got, s)
	}
	want := []string{"b", "a", "b", "a", "c", "c", "c", "b", "c", "b"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("pick %d = %q, want %q (full: %v)", i, got[i], want[i], got)
			break
		}
	}
}

func TestGenerateExcludesTaggedAlternativesWithoutAvailableTags(t *testing.T) {
	g := &Grammar{rules: map[string]*Rule{
		"snack": {
			Forms: []FormSpec{{Name: "default"}},
			Alternatives: []Alternative{
				{Weight: 1, Tags: []string{"fruit"}, Forms: map[string]Template{"default": {Literal{Text: "apple"}}}},
			},
		},
	}}
	_, err := g.Generate("snack", newRand(1))
	if err == nil {
		t.Fatal("Generate returned nil error")
	}
	if !strings.Contains(err.Error(), "snack") || !strings.Contains(err.Error(), "tags") {
		t.Fatalf("err = %v, want rule name and tags", err)
	}
}

func TestGenerateWithTagsIncludesTaggedAlternatives(t *testing.T) {
	g := &Grammar{rules: map[string]*Rule{
		"snack": {
			Forms: []FormSpec{{Name: "default"}},
			Alternatives: []Alternative{
				{Weight: 1, Tags: []string{"fruit-🍎"}, Forms: map[string]Template{"default": {Literal{Text: "apple"}}}},
			},
		},
	}}
	out, err := g.Generate("snack", newRand(1), WithTags("fruit-🍎"))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if out != "apple" {
		t.Fatalf("out = %q, want apple", out)
	}
}

func TestGenerateWithRequiredTagsRetriesUntilDirectTagProduced(t *testing.T) {
	g := &Grammar{rules: map[string]*Rule{
		"snack": {
			Forms: []FormSpec{{Name: "default"}},
			Alternatives: []Alternative{
				{Weight: 50, Forms: map[string]Template{"default": {Literal{Text: "bread"}}}},
				{Weight: 1, Tags: []string{"rare!"}, Forms: map[string]Template{"default": {Literal{Text: "apple"}}}},
			},
		},
	}}
	for seed := int64(1); seed <= 20; seed++ {
		out, err := g.Generate("snack", newRand(seed), WithRequiredTags("rare!"))
		if err != nil {
			t.Fatalf("Generate seed %d: %v", seed, err)
		}
		if out != "apple" {
			t.Fatalf("seed %d out = %q, want apple", seed, out)
		}
	}
}

func TestGenerateWithRequiredTagsSeesNestedTags(t *testing.T) {
	g := &Grammar{rules: map[string]*Rule{
		"snack": {
			Forms: []FormSpec{{Name: "default"}},
			Alternatives: []Alternative{
				{Weight: 1, Tags: []string{"fruit"}, Forms: map[string]Template{"default": {Literal{Text: "apple"}}}},
			},
		},
		"meal": {
			Forms: []FormSpec{{Name: "default"}},
			Alternatives: []Alternative{
				{Weight: 50, Forms: map[string]Template{"default": {Literal{Text: "toast"}}}},
				{Weight: 1, Forms: map[string]Template{"default": {Literal{Text: "with "}, RuleRef{Rule: "snack"}}}},
			},
		},
	}}
	out, err := g.Generate("meal", newRand(2), WithRequiredTags("fruit"))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if out != "with apple" {
		t.Fatalf("out = %q, want with apple", out)
	}
}

func TestGenerateWithRequiredTagsErrorsWhenTagIsNotProduced(t *testing.T) {
	g := singleAlt("snack", "bread")
	_, err := g.Generate("snack", newRand(1), WithRequiredTags("fruit"))
	if err == nil {
		t.Fatal("Generate returned nil error")
	}
	if !strings.Contains(err.Error(), "required tags") {
		t.Fatalf("err = %v, want required tags", err)
	}
}

func TestGenerateInvalidTagsReportDeterministically(t *testing.T) {
	g := singleAlt("snack", "bread")
	for range 20 {
		_, err := g.Generate("snack", newRand(1), WithTags("z_valid", "bad=tag"))
		if err == nil {
			t.Fatal("Generate returned nil error")
		}
		if !strings.Contains(err.Error(), `"bad=tag"`) {
			t.Fatalf("err = %v, want bad=tag to be reported first", err)
		}
	}
}

func TestGenerateRuleRefRecurses(t *testing.T) {
	g := &Grammar{rules: map[string]*Rule{
		"greeting": {
			Forms: []FormSpec{{Name: "default"}},
			Alternatives: []Alternative{{Weight: 1, Forms: map[string]Template{
				"default": {Literal{Text: "hi"}},
			}}},
		},
		"sentence": {
			Forms: []FormSpec{{Name: "default"}},
			Alternatives: []Alternative{{Weight: 1, Forms: map[string]Template{
				"default": {Literal{Text: "say "}, RuleRef{Rule: "greeting"}, Literal{Text: "!"}},
			}}},
		},
	}}
	out, err := g.Generate("sentence", newRand(1))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if out != "say hi!" {
		t.Errorf("out = %q, want %q", out, "say hi!")
	}
}

func TestGenerateRuleRefNonDefaultForm(t *testing.T) {
	g := &Grammar{rules: map[string]*Rule{
		"animal": {
			Forms: []FormSpec{
				{Name: "default"},
				{Name: "plural", Default: Template{SelfRef{}, Literal{Text: "s"}}},
			},
			Alternatives: []Alternative{{Weight: 1, Forms: map[string]Template{
				"default": {Literal{Text: "cat"}},
			}}},
		},
		"sentence": {
			Forms: []FormSpec{{Name: "default"}},
			Alternatives: []Alternative{{Weight: 1, Forms: map[string]Template{
				"default": {Literal{Text: "two "}, RuleRef{Rule: "animal", Form: "plural"}},
			}}},
		},
	}}
	out, err := g.Generate("sentence", newRand(1))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if out != "two cats" {
		t.Errorf("out = %q, want %q", out, "two cats")
	}
}

func TestGenerateRuleRefTagsAreScopedToReference(t *testing.T) {
	g := &Grammar{rules: map[string]*Rule{
		"word": {
			Forms: []FormSpec{{Name: "default"}},
			Alternatives: []Alternative{
				{Weight: 1, Tags: []string{"fruit"}, Forms: map[string]Template{"default": {Literal{Text: "apple"}}}},
			},
		},
		"sentence": {
			Forms: []FormSpec{{Name: "default"}},
			Alternatives: []Alternative{{Weight: 1, Forms: map[string]Template{
				"default": {
					RuleRef{Rule: "word", Tags: []string{"fruit"}},
					Literal{Text: " then "},
					RuleRef{Rule: "word"},
				},
			}}},
		},
	}}
	_, err := g.Generate("sentence", newRand(1))
	if err == nil {
		t.Fatal("Generate returned nil error")
	}
	if !strings.Contains(err.Error(), "word") || !strings.Contains(err.Error(), "tags") {
		t.Fatalf("err = %v, want sibling reference to lack fruit tag", err)
	}
}

func TestGenerateRuleRefTagRemovalIsScopedToReference(t *testing.T) {
	g := &Grammar{rules: map[string]*Rule{
		"word": {
			Forms: []FormSpec{{Name: "default"}},
			Alternatives: []Alternative{
				{Weight: 1, Tags: []string{"fruit"}, Forms: map[string]Template{"default": {Literal{Text: "apple"}}}},
				{Weight: 1, Forms: map[string]Template{"default": {Literal{Text: "bread"}}}},
			},
		},
		"fruit": {
			Forms: []FormSpec{{Name: "default"}},
			Alternatives: []Alternative{
				{Weight: 1, Tags: []string{"fruit"}, Forms: map[string]Template{"default": {Literal{Text: "pear"}}}},
			},
		},
		"sentence": {
			Forms: []FormSpec{{Name: "default"}},
			Alternatives: []Alternative{{Weight: 1, Forms: map[string]Template{
				"default": {
					RuleRef{Rule: "word", Tags: []string{"-fruit"}},
					Literal{Text: " then "},
					RuleRef{Rule: "fruit"},
				},
			}}},
		},
	}}
	out, err := g.Generate("sentence", newRand(1), WithTags("fruit"))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if out != "bread then pear" {
		t.Fatalf("out = %q, want bread then pear", out)
	}
}

func TestGenerateRuleRefTagsCanAddAndRemoveAvailability(t *testing.T) {
	g := &Grammar{rules: map[string]*Rule{
		"filling": {
			Forms: []FormSpec{{Name: "default"}},
			Alternatives: []Alternative{
				{Weight: 1, Tags: []string{"fruit"}, Forms: map[string]Template{"default": {Literal{Text: "apple"}}}},
				{Weight: 1, Tags: []string{"spicy"}, Forms: map[string]Template{"default": {Literal{Text: "pepper"}}}},
			},
		},
		"meal": {
			Forms: []FormSpec{{Name: "default"}},
			Alternatives: []Alternative{{Weight: 1, Forms: map[string]Template{
				"default": {RuleRef{Rule: "filling", Tags: []string{"fruit", "-spicy"}}},
			}}},
		},
	}}
	out, err := g.Generate("meal", newRand(1), WithTags("spicy"))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if out != "apple" {
		t.Fatalf("out = %q, want apple", out)
	}
}

func TestGenerateRejectsInvalidRuleRefTagRemoval(t *testing.T) {
	g := &Grammar{rules: map[string]*Rule{
		"thing": {
			Forms:        []FormSpec{{Name: "default"}},
			Alternatives: []Alternative{{Weight: 1, Forms: map[string]Template{"default": {Literal{Text: "thing"}}}}},
		},
		"sentence": {
			Forms: []FormSpec{{Name: "default"}},
			Alternatives: []Alternative{{Weight: 1, Forms: map[string]Template{
				"default": {RuleRef{Rule: "thing", Tags: []string{"-"}}},
			}}},
		},
	}}
	_, err := g.Generate("sentence", newRand(1))
	if err == nil {
		t.Fatal("Generate returned nil error")
	}
	if !strings.Contains(err.Error(), "invalid tag") {
		t.Fatalf("err = %v, want invalid tag", err)
	}
}

func TestGenerateRuleRefTagsExposeNestedAlternativesAndPropagateProducedTags(t *testing.T) {
	g := &Grammar{rules: map[string]*Rule{
		"filling": {
			Forms: []FormSpec{{Name: "default"}},
			Alternatives: []Alternative{
				{Weight: 1, Tags: []string{"mood/happy"}, Forms: map[string]Template{"default": {Literal{Text: "apple"}}}},
			},
		},
		"meal": {
			Forms: []FormSpec{{Name: "default"}},
			Alternatives: []Alternative{{Weight: 1, Forms: map[string]Template{
				"default": {Literal{Text: "pie with "}, RuleRef{Rule: "filling", Tags: []string{"mood/happy"}}},
			}}},
		},
	}}
	out, err := g.Generate("meal", newRand(1), WithRequiredTags("mood/happy"))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if out != "pie with apple" {
		t.Fatalf("out = %q, want pie with apple", out)
	}
}

func TestGenerateRuleRefRequiredRetriesNestedExpansion(t *testing.T) {
	g := &Grammar{rules: map[string]*Rule{
		"prize": {
			Forms: []FormSpec{{Name: "default"}},
			Alternatives: []Alternative{
				{Weight: 50, Forms: map[string]Template{"default": {Literal{Text: "stone"}}}},
				{Weight: 1, Tags: []string{"rare"}, Forms: map[string]Template{"default": {Literal{Text: "gem"}}}},
			},
		},
		"box": {
			Forms: []FormSpec{{Name: "default"}},
			Alternatives: []Alternative{{Weight: 1, Forms: map[string]Template{
				"default": {Literal{Text: "box: "}, RuleRef{Rule: "prize", Required: []string{"rare"}}},
			}}},
		},
	}}
	out, err := g.Generate("box", newRand(1))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if out != "box: gem" {
		t.Fatalf("out = %q, want box: gem", out)
	}
}

func TestGenerateRuleRefRequiredErrorsWhenNotProduced(t *testing.T) {
	g := &Grammar{rules: map[string]*Rule{
		"prize": {
			Forms: []FormSpec{{Name: "default"}},
			Alternatives: []Alternative{
				{Weight: 1, Forms: map[string]Template{"default": {Literal{Text: "stone"}}}},
			},
		},
		"box": {
			Forms: []FormSpec{{Name: "default"}},
			Alternatives: []Alternative{{Weight: 1, Forms: map[string]Template{
				"default": {Literal{Text: "box: "}, RuleRef{Rule: "prize", Required: []string{"rare"}}},
			}}},
		},
	}}
	_, err := g.Generate("box", newRand(1))
	if err == nil {
		t.Fatal("Generate returned nil error")
	}
	if !strings.Contains(err.Error(), "required tags") || !strings.Contains(err.Error(), "prize") {
		t.Fatalf("err = %v, want required tags on prize", err)
	}
}

func TestGenerateRuleRefRequiredCountsTagsProducedInsideReference(t *testing.T) {
	g := &Grammar{rules: map[string]*Rule{
		"fruit": {
			Forms: []FormSpec{{Name: "default"}},
			Alternatives: []Alternative{
				{Weight: 1, Tags: []string{"fruit"}, Forms: map[string]Template{"default": {Literal{Text: "apple"}}}},
			},
		},
		"meal": {
			Forms: []FormSpec{{Name: "default"}},
			Alternatives: []Alternative{{Weight: 1, Forms: map[string]Template{
				"default": {
					RuleRef{Rule: "fruit", Tags: []string{"fruit"}},
					Literal{Text: " and "},
					RuleRef{Rule: "fruit", Required: []string{"fruit"}},
				},
			}}},
		},
	}}
	out, err := g.Generate("meal", newRand(1))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if out != "apple and apple" {
		t.Fatalf("out = %q, want apple and apple", out)
	}
}

func TestGenerateNestedRuleRefRequiredDoesNotLeakDiscardedAttemptTags(t *testing.T) {
	g := &Grammar{rules: map[string]*Rule{
		"item": {
			Forms: []FormSpec{{Name: "default"}},
			Alternatives: []Alternative{
				{Weight: 1, Tags: []string{"red"}, Forms: map[string]Template{"default": {Literal{Text: "red stone"}}}},
				{Weight: 1, Tags: []string{"rare"}, Forms: map[string]Template{"default": {Literal{Text: "gem"}}}},
			},
		},
		"wrapper": {
			Forms: []FormSpec{{Name: "default"}},
			Alternatives: []Alternative{{Weight: 1, Forms: map[string]Template{
				"default": {RuleRef{Rule: "item", Required: []string{"rare"}}},
			}}},
		},
		"box": {
			Forms: []FormSpec{{Name: "default"}},
			Alternatives: []Alternative{{Weight: 1, Forms: map[string]Template{
				"default": {RuleRef{Rule: "wrapper", Required: []string{"red"}}},
			}}},
		},
	}}
	_, err := g.Generate("box", newRand(2))
	if err == nil {
		t.Fatal("Generate returned nil error")
	}
	if !strings.Contains(err.Error(), "wrapper") || !strings.Contains(err.Error(), "required tags") {
		t.Fatalf("err = %v, want wrapper required-tags failure", err)
	}
}

func TestGenerateBacktracksWhenNestedTagsMakeAlternativeImpossible(t *testing.T) {
	g := &Grammar{rules: map[string]*Rule{
		"filling": {
			Forms: []FormSpec{{Name: "default"}},
			Alternatives: []Alternative{
				{Weight: 1, Tags: []string{"fruit"}, Forms: map[string]Template{"default": {Literal{Text: "apple"}}}},
			},
		},
		"dessert": {
			Forms: []FormSpec{{Name: "default"}},
			Alternatives: []Alternative{
				{Weight: 20, Forms: map[string]Template{"default": {Literal{Text: "bad "}, RuleRef{Rule: "filling", Tags: []string{"savory"}}}}},
				{Weight: 1, Forms: map[string]Template{"default": {Literal{Text: "good "}, RuleRef{Rule: "filling", Tags: []string{"fruit"}}}}},
			},
		},
	}}
	for seed := int64(1); seed <= 20; seed++ {
		out, err := g.Generate("dessert", newRand(seed))
		if err != nil {
			t.Fatalf("Generate seed %d: %v", seed, err)
		}
		if out != "good apple" {
			t.Fatalf("seed %d out = %q, want good apple", seed, out)
		}
	}
}

func TestGenerateBacktrackingDoesNotLeakSavedNamesOrTags(t *testing.T) {
	g := &Grammar{rules: map[string]*Rule{
		"filling": {
			Forms: []FormSpec{{Name: "default"}},
			Alternatives: []Alternative{
				{Weight: 1, Tags: []string{"fruit"}, Forms: map[string]Template{"default": {Literal{Text: "apple"}}}},
			},
		},
		"dessert": {
			Forms: []FormSpec{{Name: "default"}},
			Alternatives: []Alternative{
				{Weight: 20, Tags: []string{"bad_attempt"}, Forms: map[string]Template{"default": {
					Literal{Text: "bad "},
					RuleRef{Rule: "filling", Tags: []string{"fruit"}, Save: "BAD"},
					Literal{Text: " "},
					RuleRef{Rule: "filling", Tags: []string{"savory"}},
				}}},
				{Weight: 1, Forms: map[string]Template{"default": {Literal{Text: "good "}, RuleRef{Rule: "filling", Tags: []string{"fruit"}}}}},
			},
		},
	}}
	for seed := int64(1); seed <= 20; seed++ {
		out, err := g.Generate("dessert", newRand(seed), WithRequiredTags("fruit"))
		if err != nil {
			t.Fatalf("Generate seed %d: %v", seed, err)
		}
		if out != "good apple" {
			t.Fatalf("seed %d out = %q, want good apple", seed, out)
		}
	}
	if _, err := g.Generate("dessert", newRand(1), WithRequiredTags("bad_attempt")); err == nil {
		t.Fatal("Generate with required bad_attempt returned nil error")
	}
}

func TestGenerateBacktrackingRestoresSavedNames(t *testing.T) {
	g := &Grammar{rules: map[string]*Rule{
		"filling": {
			Forms: []FormSpec{{Name: "default"}},
			Alternatives: []Alternative{
				{Weight: 1, Tags: []string{"fruit"}, Forms: map[string]Template{"default": {Literal{Text: "apple"}}}},
			},
		},
		"dessert": {
			Forms: []FormSpec{{Name: "default"}},
			Alternatives: []Alternative{
				{Weight: 20, Forms: map[string]Template{"default": {
					RuleRef{Rule: "filling", Tags: []string{"fruit"}, Save: "BAD"},
					RuleRef{Rule: "filling", Tags: []string{"savory"}},
				}}},
				{Weight: 1, Forms: map[string]Template{"default": {Recall{Name: "BAD"}}}},
			},
		},
	}}
	_, err := g.Generate("dessert", newRand(1))
	if !errors.Is(err, ErrUnsavedRecall) {
		t.Fatalf("err = %v; want errors.Is ErrUnsavedRecall", err)
	}
}

func TestGenerateSaveRecall(t *testing.T) {
	g := &Grammar{rules: map[string]*Rule{
		"animal": {
			Forms: []FormSpec{{Name: "default"}},
			Alternatives: []Alternative{
				{Weight: 1, Forms: map[string]Template{"default": {Literal{Text: "cat"}}}},
				{Weight: 1, Forms: map[string]Template{"default": {Literal{Text: "dog"}}}},
				{Weight: 1, Forms: map[string]Template{"default": {Literal{Text: "fox"}}}},
			},
		},
		"story": {
			Forms: []FormSpec{{Name: "default"}},
			Alternatives: []Alternative{{Weight: 1, Forms: map[string]Template{
				"default": {
					Literal{Text: "the "},
					RuleRef{Rule: "animal", Save: "A"},
					Literal{Text: " met another "},
					Recall{Name: "A"},
				},
			}}},
		},
	}}
	for i := range 20 {
		out, err := g.Generate("story", newRand(int64(i)))
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		// "the X met another X" — both X's must match.
		const prefix, mid = "the ", " met another "
		rest, ok := strings.CutPrefix(out, prefix)
		if !ok {
			t.Fatalf("missing prefix: %q", out)
		}
		first, second, ok := strings.Cut(rest, mid)
		if !ok {
			t.Fatalf("missing infix: %q", out)
		}
		if first != second {
			t.Errorf("save/recall mismatch: first=%q second=%q", first, second)
		}
	}
}

func TestGenerateRecallUnknownNameErrors(t *testing.T) {
	g := &Grammar{rules: map[string]*Rule{
		"r": {
			Forms: []FormSpec{{Name: "default"}},
			Alternatives: []Alternative{{Weight: 1, Forms: map[string]Template{
				"default": {Recall{Name: "X"}},
			}}},
		},
	}}
	_, err := g.Generate("r", newRand(1))
	if err == nil {
		t.Fatal("Generate returned nil for unknown recall")
	}
	if !strings.Contains(err.Error(), "X") {
		t.Errorf("error %q should mention recall name X", err)
	}
}

func TestGenerateRecursionLimit(t *testing.T) {
	g := &Grammar{rules: map[string]*Rule{
		"loop": {
			Forms: []FormSpec{{Name: "default"}},
			Alternatives: []Alternative{{Weight: 1, Forms: map[string]Template{
				"default": {RuleRef{Rule: "loop"}},
			}}},
		},
	}}
	_, err := g.Generate("loop", newRand(1))
	if !errors.Is(err, ErrRecursionLimit) {
		t.Fatalf("err = %v; want errors.Is ErrRecursionLimit", err)
	}
}

// B2: Generate(rule, nil) must reject the nil rng cleanly.
func TestGenerateNilRngErrors(t *testing.T) {
	g := singleAlt("greeting", "hi")
	_, err := g.Generate("greeting", nil)
	if err == nil {
		t.Fatal("Generate(rule, nil) returned nil error")
	}
	if !strings.Contains(err.Error(), "rng") {
		t.Errorf("error %q should mention rng", err)
	}
}

// B8: GenerateWith propagates the same nil-rng check.
func TestGenerateWithNilRngErrors(t *testing.T) {
	g := singleAlt("greeting", "hi")
	if _, err := g.GenerateWith("greeting", nil); err == nil {
		t.Fatal("GenerateWith(rule, nil) returned nil error")
	}
}

// T2: saved scope is fresh per top-level Generate call. A rule that
// recalls a name another rule saved must not see that save when called
// directly.
func TestGenerateSavedScopeIsPerCall(t *testing.T) {
	g := NewGrammar()
	if err := g.AddRule("a", &Rule{
		Forms: []FormSpec{{Name: "default"}},
		Alternatives: []Alternative{{Weight: 1, Forms: map[string]Template{
			"default": {Literal{Text: "x"}},
		}}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := g.AddRule("saver", &Rule{
		Forms: []FormSpec{{Name: "default"}},
		Alternatives: []Alternative{{Weight: 1, Forms: map[string]Template{
			"default": {RuleRef{Rule: "a", Save: "N"}},
		}}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := g.AddRule("recaller", &Rule{
		Forms: []FormSpec{{Name: "default"}},
		Alternatives: []Alternative{{Weight: 1, Forms: map[string]Template{
			"default": {Recall{Name: "N"}},
		}}},
	}); err != nil {
		t.Fatal(err)
	}
	// First call populates "N" in its own state.
	if _, err := g.Generate("saver", newRand(1)); err != nil {
		t.Fatalf("saver: %v", err)
	}
	// Second top-level call gets a fresh state — "N" must be unsaved.
	_, err := g.Generate("recaller", newRand(1))
	if !errors.Is(err, ErrUnsavedRecall) {
		t.Fatalf("err = %v; want errors.Is ErrUnsavedRecall", err)
	}
}

// T5: a recall token that fires before its save token (template
// ordering) must error, even within a single Generate call.
func TestGenerateRecallBeforeSaveErrors(t *testing.T) {
	g := NewGrammar()
	if err := g.AddRule("a", &Rule{
		Forms: []FormSpec{{Name: "default"}},
		Alternatives: []Alternative{{Weight: 1, Forms: map[string]Template{
			"default": {Literal{Text: "x"}},
		}}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := g.AddRule("story", &Rule{
		Forms: []FormSpec{{Name: "default"}},
		Alternatives: []Alternative{{Weight: 1, Forms: map[string]Template{
			"default": {
				Recall{Name: "N"},
				Literal{Text: " then "},
				RuleRef{Rule: "a", Save: "N"},
			},
		}}},
	}); err != nil {
		t.Fatal(err)
	}
	_, err := g.Generate("story", newRand(1))
	if !errors.Is(err, ErrUnsavedRecall) {
		t.Fatalf("err = %v; want errors.Is ErrUnsavedRecall", err)
	}
}

// B1: programmatic construction via the public NewGrammar / AddRule API.
func TestProgrammaticConstructionGenerates(t *testing.T) {
	g := NewGrammar()
	if err := g.AddRule("greeting", &Rule{
		Forms: []FormSpec{{Name: "default"}},
		Alternatives: []Alternative{
			{Weight: 1, Forms: map[string]Template{"default": {Literal{Text: "hi"}}}},
			{Weight: 1, Forms: map[string]Template{"default": {Literal{Text: "hey"}}}},
		},
	}); err != nil {
		t.Fatalf("AddRule: %v", err)
	}
	out, err := g.Generate("greeting", newRand(1))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if out != "hi" && out != "hey" {
		t.Errorf("out = %q, want hi or hey", out)
	}
}

// B1: AddRule rejects a duplicate name and validates rule shape.
func TestAddRuleValidatesShape(t *testing.T) {
	g := NewGrammar()
	if err := g.AddRule("x", &Rule{
		Forms: []FormSpec{{Name: "default"}},
		Alternatives: []Alternative{{Weight: 1, Forms: map[string]Template{
			"default": {Literal{Text: "a"}},
		}}},
	}); err != nil {
		t.Fatal(err)
	}
	// Duplicate name.
	dup := g.AddRule("x", &Rule{
		Forms:        []FormSpec{{Name: "default"}},
		Alternatives: []Alternative{{Weight: 1, Forms: map[string]Template{"default": {Literal{Text: "b"}}}}},
	})
	if !errors.Is(dup, ErrDuplicateRule) {
		t.Errorf("duplicate err = %v; want errors.Is ErrDuplicateRule", dup)
	}
	// Default form with a default template.
	bad := g.AddRule("y", &Rule{
		Forms:        []FormSpec{{Name: "default", Default: Template{Literal{Text: "z"}}}},
		Alternatives: []Alternative{{Weight: 1, Forms: map[string]Template{"default": {Literal{Text: "a"}}}}},
	})
	if bad == nil {
		t.Errorf("expected error for default form with default template")
	}
	// Alternative missing the default form.
	bad = g.AddRule("z", &Rule{
		Forms:        []FormSpec{{Name: "default"}},
		Alternatives: []Alternative{{Weight: 1, Forms: map[string]Template{"plural": {Literal{Text: "a"}}}}},
	})
	if bad == nil {
		t.Errorf("expected error for alternative missing default form")
	}
}

// Q5: AAn at end of string after a preceding word.
// (Both branches: ending without a following word stays unchanged;
// followed by a vowel-starting word gets the rewrite.)
func TestParseAndGenerateAAnEdgeCases(t *testing.T) {
	// This is a parse/generate composition test; the AAn-only behaviour
	// is exercised in the english package. Here we just sanity-check
	// that the grammar surface is usable end-to-end for the same case.
	src := `rule s
  forms: default
  foo a
  foo a apple
`
	g, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	for range 10 {
		out, err := g.Generate("s", newRand(1))
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		if out != "foo a" && out != "foo a apple" {
			t.Errorf("unexpected: %q", out)
		}
	}
}

func TestGenerateRuleRefUnknownFormErrors(t *testing.T) {
	g := singleAlt("greeting", "hi")
	g.rules["caller"] = &Rule{
		Forms: []FormSpec{{Name: "default"}},
		Alternatives: []Alternative{{Weight: 1, Forms: map[string]Template{
			"default": {RuleRef{Rule: "greeting", Form: "plural"}},
		}}},
	}
	_, err := g.Generate("caller", newRand(1))
	if err == nil {
		t.Fatal("expected error for missing form")
	}
	if !strings.Contains(err.Error(), "plural") {
		t.Errorf("error %q should name the missing form", err)
	}
}
