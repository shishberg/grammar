package grammar

import (
	"fmt"
	"math/rand"
	"strings"
)

// defaultMaxDepth caps how deep the generator will recurse before giving
// up. A pathological grammar (rule that always references itself) would
// otherwise grow the stack until the runtime kills it. The cap counts
// rule expansions, not template tokens, so it tracks the user-visible
// nesting depth rather than the byte length of the output.
const defaultMaxDepth = 200

// PostProcessor transforms generated output. Hosts compose them via
// GenerateWith; the package ships English helpers in a subpackage but
// hosts targeting other languages can leave them off.
type PostProcessor func(string) string

// GenerateWith generates the rule and then applies post in declaration
// order. With no post-processors it behaves like Generate.
func (g *Grammar) GenerateWith(rule string, rng *rand.Rand, post ...PostProcessor) (string, error) {
	out, err := g.Generate(rule, rng)
	if err != nil {
		return "", err
	}
	for _, p := range post {
		out = p(out)
	}
	return out, nil
}

// Generate produces one expansion of the named rule's default form.
func (g *Grammar) Generate(rule string, rng *rand.Rand) (string, error) {
	if rng == nil {
		return "", fmt.Errorf("grammar: rng must not be nil")
	}
	r, ok := g.rules[rule]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUndefinedRule, rule)
	}
	st := &genState{
		grammar: g,
		rng:     rng,
		saved:   map[string]string{},
		max:     defaultMaxDepth,
	}
	var sb strings.Builder
	if err := st.expandRule(&sb, rule, r, "", 0); err != nil {
		return "", err
	}
	return sb.String(), nil
}

// genState is the per-Generate-call mutable context: the rng, saved
// variables visible across rule boundaries, and the recursion depth
// budget. Each top-level Generate call gets a fresh genState so saved
// names don't leak across calls.
type genState struct {
	grammar *Grammar
	rng     *rand.Rand
	saved   map[string]string
	max     int
}

func (s *genState) expandRule(out *strings.Builder, name string, r *Rule, formName string, depth int) error {
	if depth > s.max {
		return fmt.Errorf("%w (%d) at rule %q", ErrRecursionLimit, s.max, name)
	}
	if formName == "" {
		formName = r.Forms[0].Name
	}
	form, ok := findForm(r, formName)
	if !ok {
		return fmt.Errorf("rule %q does not declare %w %q", name, ErrUnknownForm, formName)
	}
	alt, err := s.pickAlternative(name, r)
	if err != nil {
		return err
	}
	// SelfRef inside form.Default substitutes the alternative's
	// default-form expansion; the form-default branch handles that.
	tpl, ok := alt.Forms[formName]
	if !ok {
		if form.Default == nil {
			return fmt.Errorf("rule %q alternative does not supply form %q and the form has no default", name, formName)
		}
		return s.expandFormDefault(out, name, r, alt, form.Default, depth)
	}
	return s.expandTemplate(out, name, tpl, depth, "")
}

// expandFormDefault expands a form-default template. SelfRef inside it
// is replaced by the alternative's default-form expansion, which is
// computed once and cached so a default template that mentions {} more
// than once stays consistent.
func (s *genState) expandFormDefault(out *strings.Builder, name string, r *Rule, alt *Alternative, tpl Template, depth int) error {
	defaultName := r.Forms[0].Name
	defaultTpl, ok := alt.Forms[defaultName]
	if !ok {
		return fmt.Errorf("rule %q alternative missing default form %q", name, defaultName)
	}
	var selfBuf strings.Builder
	if err := s.expandTemplate(&selfBuf, name, defaultTpl, depth, ""); err != nil {
		return err
	}
	return s.expandTemplate(out, name, tpl, depth, selfBuf.String())
}

// expandTemplate writes one template's tokens to out. selfText is the
// substitution for SelfRef; empty if SelfRef has no meaning here (that
// case is rejected when SelfRef is encountered).
func (s *genState) expandTemplate(out *strings.Builder, name string, tpl Template, depth int, selfText string) error {
	for _, tok := range tpl {
		switch t := tok.(type) {
		case Literal:
			out.WriteString(t.Text)
		case RuleRef:
			sub, ok := s.grammar.rules[t.Rule]
			if !ok {
				return fmt.Errorf("rule %q references %w %q", name, ErrUndefinedRule, t.Rule)
			}
			var subBuf strings.Builder
			if err := s.expandRule(&subBuf, t.Rule, sub, t.Form, depth+1); err != nil {
				return err
			}
			expansion := subBuf.String()
			if t.Save != "" {
				s.saved[t.Save] = expansion
			}
			out.WriteString(expansion)
		case Recall:
			v, ok := s.saved[t.Name]
			if !ok {
				return fmt.Errorf("rule %q: %w %q", name, ErrUnsavedRecall, t.Name)
			}
			out.WriteString(v)
		case SelfRef:
			if selfText == "" {
				// SelfRef outside a form-default template is a grammar
				// construction bug; the parser refuses it but a hand-built
				// grammar can still hit this branch.
				return fmt.Errorf("rule %q uses self-ref outside a form-default template", name)
			}
			out.WriteString(selfText)
		default:
			return fmt.Errorf("internal: rule %q has unknown token type %T", name, tok)
		}
	}
	return nil
}

// pickAlternative selects one alternative from r weighted by the
// alternatives' Weight fields. Weight 0 is normalised to 1 so a
// hand-built grammar that forgets to set Weight still works; Parse
// rejects an explicit weight=0 separately.
func (s *genState) pickAlternative(name string, r *Rule) (*Alternative, error) {
	if len(r.Alternatives) == 0 {
		return nil, fmt.Errorf("rule %q has no alternatives", name)
	}
	var total uint
	for i := range r.Alternatives {
		w := r.Alternatives[i].Weight
		if w == 0 {
			w = 1
		}
		total += w
	}
	if total == 0 {
		return nil, fmt.Errorf("rule %q has zero total weight", name)
	}
	pick := uint(s.rng.Int63n(int64(total)))
	for i := range r.Alternatives {
		w := r.Alternatives[i].Weight
		if w == 0 {
			w = 1
		}
		if pick < w {
			return &r.Alternatives[i], nil
		}
		pick -= w
	}
	return nil, fmt.Errorf("internal: weighted pick fell through (rule %q, total %d)", name, total)
}

func findForm(r *Rule, name string) (FormSpec, bool) {
	for _, f := range r.Forms {
		if f.Name == name {
			return f, true
		}
	}
	return FormSpec{}, false
}
