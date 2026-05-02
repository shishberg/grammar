package grammar

import (
	"errors"
	"fmt"
	"maps"
	"math/rand"
	"slices"
	"strings"
)

// defaultMaxDepth caps how deep the generator will recurse before giving
// up. A pathological grammar (rule that always references itself) would
// otherwise grow the stack until the runtime kills it. The cap counts
// rule expansions, not template tokens, so it tracks the user-visible
// nesting depth rather than the byte length of the output.
const defaultMaxDepth = 200
const defaultRequiredTagAttempts = 1000

var (
	errNoEligibleAlternatives = errors.New("no eligible alternatives")
	errRequiredTagsMissing    = errors.New("required tags not produced")
)

// PostProcessor transforms generated output. Hosts compose them via
// GenerateWith; the package ships English helpers in a subpackage but
// hosts targeting other languages can leave them off.
type PostProcessor func(string) string

// GenerateOption configures tag availability and requirements for one
// Generate call.
type GenerateOption func(*generateConfig)

type generateConfig struct {
	available map[string]bool
	required  map[string]bool
}

// WithTags makes tagged alternatives eligible for this generation.
func WithTags(tags ...string) GenerateOption {
	return func(c *generateConfig) {
		for _, tag := range tags {
			c.available[tag] = true
		}
	}
}

// WithRequiredTags requires the finished top-level expansion to have
// produced each tag. Required tags are also available while generating.
func WithRequiredTags(tags ...string) GenerateOption {
	return func(c *generateConfig) {
		for _, tag := range tags {
			c.available[tag] = true
			c.required[tag] = true
		}
	}
}

// GenerateWith generates the rule and then applies post in declaration
// order. With no post-processors it behaves like Generate.
func (g *Grammar) GenerateWith(rule string, rng *rand.Rand, post ...PostProcessor) (string, error) {
	return g.GenerateWithOptions(rule, rng, nil, post...)
}

// GenerateWithOptions generates the rule with opts and then applies post
// in declaration order. It is the option-aware form of GenerateWith for
// callers that need tags and post-processors together.
func (g *Grammar) GenerateWithOptions(rule string, rng *rand.Rand, opts []GenerateOption, post ...PostProcessor) (string, error) {
	out, err := g.Generate(rule, rng, opts...)
	if err != nil {
		return "", err
	}
	for _, p := range post {
		out = p(out)
	}
	return out, nil
}

// Generate produces one expansion of the named rule's default form.
func (g *Grammar) Generate(rule string, rng *rand.Rand, opts ...GenerateOption) (string, error) {
	if rng == nil {
		return "", fmt.Errorf("grammar: rng must not be nil")
	}
	r, ok := g.rules[rule]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUndefinedRule, rule)
	}
	cfg := generateConfig{
		available: map[string]bool{},
		required:  map[string]bool{},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	if err := validateTagSet(cfg.available); err != nil {
		return "", err
	}
	if err := validateTagSet(cfg.required); err != nil {
		return "", err
	}
	attempts := 1
	if len(cfg.required) > 0 {
		attempts = defaultRequiredTagAttempts
	}
	var last string
	for attempt := 0; attempt < attempts; attempt++ {
		out, produced, err := g.generateOnce(rule, r, rng, cfg.available)
		if err != nil {
			return "", err
		}
		if hasAllTags(produced, cfg.required) {
			return out, nil
		}
		last = out
	}
	return "", fmt.Errorf("grammar: rule %q could not produce required tags after %d attempts (last output %q)", rule, attempts, last)
}

func (g *Grammar) generateOnce(rule string, r *Rule, rng *rand.Rand, available map[string]bool) (string, map[string]bool, error) {
	st := &genState{
		grammar:   g,
		rng:       rng,
		saved:     map[string]string{},
		available: available,
		produced:  map[string]bool{},
		max:       defaultMaxDepth,
	}
	var sb strings.Builder
	if err := st.expandRule(&sb, rule, r, "", 0); err != nil {
		return "", nil, err
	}
	return sb.String(), st.produced, nil
}

// genState is the per-Generate-call mutable context: the rng, saved
// variables visible across rule boundaries, and the recursion depth
// budget. Each top-level Generate call gets a fresh genState so saved
// names don't leak across calls.
type genState struct {
	grammar   *Grammar
	rng       *rand.Rand
	saved     map[string]string
	available map[string]bool
	produced  map[string]bool
	scopes    []map[string]bool
	max       int
}

type genSnapshot struct {
	saved     map[string]string
	available map[string]bool
	produced  map[string]bool
	scopes    []map[string]bool
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
	if len(r.Alternatives) == 0 {
		return fmt.Errorf("rule %q has no alternatives", name)
	}
	remaining := s.eligibleAlternatives(r)
	if len(remaining) == 0 {
		return fmt.Errorf("rule %q has no eligible alternatives for available tags: %w", name, errNoEligibleAlternatives)
	}
	var lastErr error
	for len(remaining) > 0 {
		picked := s.pickAlternativeIndex(r, remaining)
		alt := &r.Alternatives[picked]
		snapshot := s.snapshot()
		var buf strings.Builder
		if err := s.expandAlternative(&buf, name, r, form, alt, formName, depth); err != nil {
			s.restore(snapshot)
			if !isBacktrackable(err) {
				return err
			}
			lastErr = err
			remaining = removeAlternativeIndex(remaining, picked)
			continue
		}
		out.WriteString(buf.String())
		return nil
	}
	return fmt.Errorf("rule %q: all eligible alternatives failed: %w", name, lastErr)
}

func (s *genState) expandAlternative(out *strings.Builder, name string, r *Rule, form FormSpec, alt *Alternative, formName string, depth int) error {
	for _, tag := range alt.Tags {
		s.produced[tag] = true
		for _, scope := range s.scopes {
			scope[tag] = true
		}
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

func (s *genState) snapshot() genSnapshot {
	return genSnapshot{
		saved:     maps.Clone(s.saved),
		available: s.available,
		produced:  maps.Clone(s.produced),
		scopes:    cloneScopeStack(s.scopes),
	}
}

func (s *genState) restore(snapshot genSnapshot) {
	s.saved = snapshot.saved
	s.available = snapshot.available
	s.produced = snapshot.produced
	restoreScopeStack(s.scopes, snapshot.scopes)
}

func isBacktrackable(err error) bool {
	return errors.Is(err, errNoEligibleAlternatives) || errors.Is(err, errRequiredTagsMissing)
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
			expansion, err := s.expandRuleRef(name, t, depth)
			if err != nil {
				return err
			}
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

func (s *genState) expandRuleRef(caller string, ref RuleRef, depth int) (string, error) {
	if err := validateRuleRefTags(ref); err != nil {
		return "", fmt.Errorf("rule %q reference to %q: %w", caller, ref.Rule, err)
	}
	sub, ok := s.grammar.rules[ref.Rule]
	if !ok {
		return "", fmt.Errorf("rule %q references %w %q", caller, ErrUndefinedRule, ref.Rule)
	}
	if len(ref.Required) == 0 {
		var subBuf strings.Builder
		available := s.available
		s.available = withAvailableTags(s.available, ref.Tags, nil)
		err := s.expandRule(&subBuf, ref.Rule, sub, ref.Form, depth+1)
		s.available = available
		if err != nil {
			return "", err
		}
		return subBuf.String(), nil
	}

	available := withAvailableTags(s.available, ref.Tags, ref.Required)
	for attempt := 0; attempt < defaultRequiredTagAttempts; attempt++ {
		saved := maps.Clone(s.saved)
		produced := maps.Clone(s.produced)
		scopes := cloneScopeStack(s.scopes)
		previousAvailable := s.available
		scope := map[string]bool{}
		s.available = available
		s.scopes = append(s.scopes, scope)
		var subBuf strings.Builder
		err := s.expandRule(&subBuf, ref.Rule, sub, ref.Form, depth+1)
		s.scopes = s.scopes[:len(s.scopes)-1]
		s.available = previousAvailable
		if err != nil {
			s.saved = saved
			s.produced = produced
			restoreScopeStack(s.scopes, scopes)
			return "", err
		}
		if hasAllTags(scope, tagsToSet(ref.Required)) {
			return subBuf.String(), nil
		}
		s.saved = saved
		s.produced = produced
		restoreScopeStack(s.scopes, scopes)
	}
	return "", fmt.Errorf("grammar: rule %q could not produce required tags after %d attempts: %w", ref.Rule, defaultRequiredTagAttempts, errRequiredTagsMissing)
}

// eligibleAlternatives returns the alternatives whose tag prerequisites
// are satisfied by the current generation context.
func (s *genState) eligibleAlternatives(r *Rule) []int {
	eligible := make([]int, 0, len(r.Alternatives))
	for i := range r.Alternatives {
		if s.tagsAvailable(r.Alternatives[i].Tags) {
			eligible = append(eligible, i)
		}
	}
	return eligible
}

// pickAlternativeIndex selects one candidate weighted by the alternatives'
// Weight fields. Weight 0 is normalised to 1 so a hand-built grammar that
// forgets to set Weight still works; Parse rejects an explicit weight=0
// separately.
func (s *genState) pickAlternativeIndex(r *Rule, candidates []int) int {
	var total uint
	for _, i := range candidates {
		w := r.Alternatives[i].Weight
		if w == 0 {
			w = 1
		}
		total += w
	}
	pick := uint(s.rng.Int63n(int64(total)))
	for _, i := range candidates {
		w := r.Alternatives[i].Weight
		if w == 0 {
			w = 1
		}
		if pick < w {
			return i
		}
		pick -= w
	}
	panic(fmt.Sprintf("internal: weighted pick fell through (total %d)", total))
}

func removeAlternativeIndex(candidates []int, picked int) []int {
	for i, candidate := range candidates {
		if candidate == picked {
			return append(candidates[:i], candidates[i+1:]...)
		}
	}
	return candidates
}

func (s *genState) tagsAvailable(tags []string) bool {
	for _, tag := range tags {
		if !s.available[tag] {
			return false
		}
	}
	return true
}

func hasAllTags(produced, required map[string]bool) bool {
	for tag := range required {
		if !produced[tag] {
			return false
		}
	}
	return true
}

func withAvailableTags(base map[string]bool, tags, required []string) map[string]bool {
	if len(tags) == 0 && len(required) == 0 {
		return base
	}
	out := maps.Clone(base)
	for _, tag := range tags {
		out[tag] = true
	}
	for _, tag := range required {
		out[tag] = true
	}
	return out
}

func tagsToSet(tags []string) map[string]bool {
	out := make(map[string]bool, len(tags))
	for _, tag := range tags {
		out[tag] = true
	}
	return out
}

func cloneScopeStack(scopes []map[string]bool) []map[string]bool {
	out := make([]map[string]bool, len(scopes))
	for i, scope := range scopes {
		out[i] = maps.Clone(scope)
	}
	return out
}

func restoreScopeStack(scopes, snapshots []map[string]bool) {
	for i, snapshot := range snapshots {
		clear(scopes[i])
		for tag, produced := range snapshot {
			scopes[i][tag] = produced
		}
	}
}

func validateTagSet(tags map[string]bool) error {
	for _, tag := range slices.Sorted(maps.Keys(tags)) {
		if !isTagName(tag) {
			return fmt.Errorf("grammar: invalid tag %q (%s)", tag, invalidTagDescription)
		}
	}
	return nil
}

func findForm(r *Rule, name string) (FormSpec, bool) {
	for _, f := range r.Forms {
		if f.Name == name {
			return f, true
		}
	}
	return FormSpec{}, false
}
