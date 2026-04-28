package grammar

import (
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

func TestMergeOverlappingErrors(t *testing.T) {
	g := singleAlt("a", "alpha")
	other := singleAlt("a", "also-alpha")
	err := g.Merge(other)
	if err == nil {
		t.Fatal("Merge of overlapping rule did not error")
	}
	if !strings.Contains(err.Error(), "a") {
		t.Errorf("error %q should name the colliding rule", err)
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
