// Package grammar parses and generates text from a rule-based grammar.
//
// The package is designed to be lifted into its own module later, so it
// has no dependencies on its host project.
package grammar

import (
	"errors"
	"fmt"
	"reflect"
)

// Sentinel errors. Callers can match these with errors.Is to distinguish
// the kinds of failure they want to react to.
var (
	ErrUndefinedRule      = errors.New("undefined rule")
	ErrUnknownForm        = errors.New("unknown form")
	ErrUnsavedRecall      = errors.New("recall of unsaved name")
	ErrRecursionLimit     = errors.New("recursion limit exceeded")
	ErrDuplicateRule      = errors.New("duplicate rule")
	ErrFormSchemeMismatch = errors.New("form scheme mismatch")
)

// Grammar is a set of named rules. Construct one with Parse or NewGrammar;
// programmatic callers populate Grammar via AddRule using the exported
// Rule, FormSpec, Alternative, and Template types.
type Grammar struct {
	rules map[string]*Rule
}

// NewGrammar returns an empty Grammar that callers can populate via AddRule.
func NewGrammar() *Grammar {
	return &Grammar{rules: map[string]*Rule{}}
}

// AddRule installs r under name. Returns an error if name is already
// defined or if r fails the same shape checks Parse applies (the default
// form must be present and must not have a default template).
func (g *Grammar) AddRule(name string, r *Rule) error {
	if g.rules == nil {
		g.rules = map[string]*Rule{}
	}
	if _, exists := g.rules[name]; exists {
		return fmt.Errorf("%w: %q", ErrDuplicateRule, name)
	}
	if r == nil {
		return fmt.Errorf("rule %q: nil rule", name)
	}
	if len(r.Forms) == 0 {
		return fmt.Errorf("rule %q: no forms declared", name)
	}
	if r.Forms[0].Default != nil {
		return fmt.Errorf("rule %q: default form %q must not have a default template", name, r.Forms[0].Name)
	}
	if len(r.Alternatives) == 0 {
		return fmt.Errorf("rule %q: no alternatives", name)
	}
	defaultName := r.Forms[0].Name
	for i, alt := range r.Alternatives {
		if _, ok := alt.Forms[defaultName]; !ok {
			return fmt.Errorf("rule %q alternative %d: missing default form %q", name, i, defaultName)
		}
	}
	g.rules[name] = r
	return nil
}

// Rule is one named expansion site in a grammar. It declares one or more
// inflectional forms (the first is the default) and a list of weighted
// alternative expansions.
type Rule struct {
	Forms        []FormSpec
	Alternatives []Alternative
}

// FormSpec describes one inflectional form of a rule. Default is the
// template applied to alternatives that don't supply this form's value;
// SelfRef inside Default substitutes the alternative's default-form
// expansion. The default form (index 0 of Rule.Forms) must leave Default
// empty — every alternative supplies the default form directly.
type FormSpec struct {
	Name    string
	Default Template
}

// Alternative is one weighted choice within a rule. Forms maps form name
// to the template used when that form is requested. A zero Weight is
// treated as 1 by the generator; Parse rejects an explicit weight=0.
type Alternative struct {
	Weight uint
	Forms  map[string]Template
}

// Template is a sequence of literal characters and substitution tokens.
type Template []Token

// Token is the sealed sum of template-element kinds. The unexported
// token() method keeps third-party packages from minting new variants.
type Token interface {
	token()
}

// Literal is plain text inside a template.
type Literal struct {
	Text string
}

// RuleRef expands the named rule. Form selects an inflectional form
// (empty means default). Save, if non-empty, stores the expansion under
// that name for later Recall in the same Generate call.
type RuleRef struct {
	Rule string
	Form string
	Save string
}

// Recall substitutes a previously saved expansion by name.
type Recall struct {
	Name string
}

// SelfRef is the empty {} placeholder. Legal only inside a non-default
// form's Default template, where it stands in for the alternative's
// default-form expansion.
type SelfRef struct{}

func (Literal) token() {}
func (RuleRef) token() {}
func (Recall) token()  {}
func (SelfRef) token() {}

// Validate walks every template in g and checks that each RuleRef
// names a rule defined in g and (when Form is set) that the target
// rule declares that form. Errors wrap ErrUndefinedRule or
// ErrUnknownForm so callers can match them with errors.Is.
//
// Parse only enforces structural correctness inside the source it
// sees, so a file with a forward reference to a rule defined in a
// sibling file Parses successfully. Call Validate after Merging or
// AddRule-ing all the pieces of a grammar together, before Generate.
// Generate itself still rejects an unresolved reference at expansion
// time, so Validate is optional — but calling it surfaces the error
// up front rather than only on the unlucky generation that happens to
// pick the broken alternative.
func (g *Grammar) Validate() error {
	for name, r := range g.rules {
		for _, alt := range r.Alternatives {
			for _, tpl := range alt.Forms {
				if err := g.validateTemplateRefs(name, tpl); err != nil {
					return err
				}
			}
		}
		for _, fs := range r.Forms {
			if err := g.validateTemplateRefs(name, fs.Default); err != nil {
				return err
			}
		}
	}
	return nil
}

func (g *Grammar) validateTemplateRefs(ruleName string, tpl Template) error {
	for _, tok := range tpl {
		ref, ok := tok.(RuleRef)
		if !ok {
			continue
		}
		target, exists := g.rules[ref.Rule]
		if !exists {
			return fmt.Errorf("rule %q references %w %q", ruleName, ErrUndefinedRule, ref.Rule)
		}
		if ref.Form == "" {
			continue
		}
		found := false
		for _, fs := range target.Forms {
			if fs.Name == ref.Form {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("rule %q references %w %q on rule %q", ruleName, ErrUnknownForm, ref.Form, ref.Rule)
		}
	}
	return nil
}

// Merge adds the rules of other into g. When both grammars define a
// rule with the same name, the two definitions are combined provided
// their form schemes match: same form names in the same order, with
// structurally equal form-default templates. The combined rule's
// alternatives are g's alternatives followed by other's, in source
// order, with weights preserved.
//
// A name collision with mismatched form schemes wraps
// ErrFormSchemeMismatch. Merge(nil) is a no-op.
func (g *Grammar) Merge(other *Grammar) error {
	if other == nil || len(other.rules) == 0 {
		return nil
	}
	if g.rules == nil {
		g.rules = make(map[string]*Rule, len(other.rules))
	}
	for name, r := range other.rules {
		existing, exists := g.rules[name]
		if !exists {
			g.rules[name] = r
			continue
		}
		if !formSchemesMatch(existing.Forms, r.Forms) {
			return fmt.Errorf("%w: rule %q", ErrFormSchemeMismatch, name)
		}
		existing.Alternatives = append(existing.Alternatives, r.Alternatives...)
	}
	return nil
}

// formSchemesMatch reports whether two Forms slices declare the same
// form names in the same order with structurally equal form-default
// templates. reflect.DeepEqual handles both the slice-equality and
// the nil-vs-empty case (the default form's Default is nil on both
// sides of any matching pair).
func formSchemesMatch(a, b []FormSpec) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name {
			return false
		}
		if !reflect.DeepEqual(a[i].Default, b[i].Default) {
			return false
		}
	}
	return true
}
