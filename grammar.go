// Package grammar parses and generates text from a rule-based grammar.
//
// The package is designed to be lifted into its own module later, so it
// has no dependencies on its host project.
package grammar

import (
	"errors"
	"fmt"
)

// Sentinel errors. Callers can match these with errors.Is to distinguish
// the kinds of failure they want to react to.
var (
	ErrUndefinedRule  = errors.New("undefined rule")
	ErrUnknownForm    = errors.New("unknown form")
	ErrUnsavedRecall  = errors.New("recall of unsaved name")
	ErrRecursionLimit = errors.New("recursion limit exceeded")
	ErrDuplicateRule  = errors.New("duplicate rule")
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

// Merge adds the rules of other into g. A rule name that already exists
// in g is an error (wraps ErrDuplicateRule) rather than a silent override.
// Merge(nil) is a no-op.
func (g *Grammar) Merge(other *Grammar) error {
	if other == nil || len(other.rules) == 0 {
		return nil
	}
	if g.rules == nil {
		g.rules = make(map[string]*Rule, len(other.rules))
	}
	for name, r := range other.rules {
		if _, exists := g.rules[name]; exists {
			return fmt.Errorf("%w: %q", ErrDuplicateRule, name)
		}
		g.rules[name] = r
	}
	return nil
}
