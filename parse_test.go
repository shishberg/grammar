package grammar

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// A rule whose only body lines are comments has no entries and must
// be reported the same way as a header followed by EOF.
func TestParseRuleWithOnlyCommentsErrors(t *testing.T) {
	src := "rule x\n  # just a comment\n  # and another\n"
	_, err := Parse(src)
	if err == nil {
		t.Fatal("Parse: want error, got nil")
	}
	if !strings.Contains(err.Error(), "no entries") {
		t.Errorf("err = %v; want it to mention 'no entries'", err)
	}
}

func TestParseEmpty(t *testing.T) {
	g, err := Parse("")
	if err != nil {
		t.Fatalf("Parse(\"\"): %v", err)
	}
	if g == nil {
		t.Fatal("Parse(\"\") returned nil grammar")
	}
	if len(g.rules) != 0 {
		t.Errorf("rules = %d, want 0", len(g.rules))
	}
}

func TestParseOneRuleOneFormMultipleEntries(t *testing.T) {
	src := `rule greeting
  forms: default
  hello
  hi
  hey
`
	g, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	r, ok := g.rules["greeting"]
	if !ok {
		t.Fatalf("rule greeting missing")
	}
	if len(r.Forms) != 1 || r.Forms[0].Name != "default" {
		t.Errorf("forms = %+v", r.Forms)
	}
	if len(r.Alternatives) != 3 {
		t.Fatalf("alternatives = %d, want 3", len(r.Alternatives))
	}
	want := []string{"hello", "hi", "hey"}
	for i, w := range want {
		got := r.Alternatives[i].Forms["default"]
		if len(got) != 1 {
			t.Fatalf("alt %d tokens = %d, want 1", i, len(got))
		}
		lit, ok := got[0].(Literal)
		if !ok {
			t.Fatalf("alt %d token is %T, want Literal", i, got[0])
		}
		if lit.Text != w {
			t.Errorf("alt %d text = %q, want %q", i, lit.Text, w)
		}
		if r.Alternatives[i].Weight != 1 {
			t.Errorf("alt %d weight = %d, want 1", i, r.Alternatives[i].Weight)
		}
	}
}

func TestParseCommentsAndBlankLines(t *testing.T) {
	src := `# top comment
rule greeting
  forms: default  # form decl
  hello   # inline comment
  # standalone comment
  hi


# trailing block

rule farewell
  forms: default
  bye
`
	g, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(g.rules) != 2 {
		t.Errorf("rules = %d, want 2", len(g.rules))
	}
	if g.rules["greeting"] == nil || len(g.rules["greeting"].Alternatives) != 2 {
		t.Errorf("greeting alts = %v", g.rules["greeting"])
	}
	if g.rules["farewell"] == nil || len(g.rules["farewell"].Alternatives) != 1 {
		t.Errorf("farewell alts = %v", g.rules["farewell"])
	}
}

func TestParseFormsDefaultTemplateAndOverride(t *testing.T) {
	src := `rule animal
  forms: default, plural={}s
  cat
  mouse | mice
`
	g, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	r := g.rules["animal"]
	if r == nil {
		t.Fatal("animal missing")
	}
	if len(r.Forms) != 2 {
		t.Fatalf("forms = %d, want 2", len(r.Forms))
	}
	if r.Forms[0].Name != "default" {
		t.Errorf("forms[0] = %q, want default", r.Forms[0].Name)
	}
	if r.Forms[1].Name != "plural" {
		t.Errorf("forms[1] = %q, want plural", r.Forms[1].Name)
	}
	wantDefault := Template{SelfRef{}, Literal{Text: "s"}}
	if !reflect.DeepEqual(r.Forms[1].Default, wantDefault) {
		t.Errorf("plural default = %#v, want %#v", r.Forms[1].Default, wantDefault)
	}
	// cat has only the default form supplied; plural falls back.
	if _, has := r.Alternatives[0].Forms["plural"]; has {
		t.Error("cat should not supply plural directly")
	}
	// mouse explicitly overrides plural.
	mousePlural := r.Alternatives[1].Forms["plural"]
	if len(mousePlural) != 1 {
		t.Fatalf("mouse plural tokens = %d, want 1", len(mousePlural))
	}
	if lit, ok := mousePlural[0].(Literal); !ok || lit.Text != "mice" {
		t.Errorf("mouse plural = %#v, want Literal{mice}", mousePlural[0])
	}
}

func TestParseWeightPrefix(t *testing.T) {
	src := `rule pick
  forms: default
  weight=2 a
  b
  weight=5 c
`
	g, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	r := g.rules["pick"]
	wantWeights := []uint{2, 1, 5}
	wantText := []string{"a", "b", "c"}
	for i, w := range wantWeights {
		if r.Alternatives[i].Weight != w {
			t.Errorf("alt %d weight = %d, want %d", i, r.Alternatives[i].Weight, w)
		}
		if lit, ok := r.Alternatives[i].Forms["default"][0].(Literal); !ok || lit.Text != wantText[i] {
			t.Errorf("alt %d text = %#v, want %q", i, r.Alternatives[i].Forms["default"][0], wantText[i])
		}
	}
}

func TestParseRuleRefForms(t *testing.T) {
	src := `rule a
  forms: default, plural={}s
  cat

rule x
  forms: default
  {a} {a:plural} {a as N} {a:plural as N} {*N}
`
	g, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	gotTpl := g.rules["x"].Alternatives[0].Forms["default"]
	wantTpl := Template{
		RuleRef{Rule: "a"},
		Literal{Text: " "},
		RuleRef{Rule: "a", Form: "plural"},
		Literal{Text: " "},
		RuleRef{Rule: "a", Save: "N"},
		Literal{Text: " "},
		RuleRef{Rule: "a", Form: "plural", Save: "N"},
		Literal{Text: " "},
		Recall{Name: "N"},
	}
	if !reflect.DeepEqual(gotTpl, wantTpl) {
		t.Errorf("template = %#v\nwant      %#v", gotTpl, wantTpl)
	}

	// The plural form's default template should match exactly. If
	// leading whitespace after '=' weren't trimmed, this would catch it.
	wantPlural := Template{SelfRef{}, Literal{Text: "s"}}
	if got := g.rules["a"].Forms[1].Default; !reflect.DeepEqual(got, wantPlural) {
		t.Errorf("a.plural default = %#v, want %#v", got, wantPlural)
	}
}

func TestParseHashInsideBraceIsLiteral(t *testing.T) {
	// A '#' inside a {...} ref body is part of the body, not a comment
	// start. The parser will then reject it as an invalid rule name,
	// which is fine — the point is that we don't truncate the line.
	src := "rule x\n  forms: default\n  hi {a#b}\n"
	_, err := Parse(src)
	if err == nil {
		t.Fatal("expected error for '#' inside ref")
	}
	if !strings.Contains(err.Error(), "rule name") && !strings.Contains(err.Error(), "a#b") {
		t.Errorf("error %q should reflect the ref body containing #", err)
	}
}

func TestParseEscapes(t *testing.T) {
	src := `rule x
  forms: default
  a\{b\}c \# not a comment
`
	g, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	tpl := g.rules["x"].Alternatives[0].Forms["default"]
	if len(tpl) != 1 {
		t.Fatalf("tokens = %d, want 1", len(tpl))
	}
	lit, ok := tpl[0].(Literal)
	if !ok {
		t.Fatalf("token is %T", tpl[0])
	}
	if lit.Text != "a{b}c # not a comment" {
		t.Errorf("text = %q", lit.Text)
	}
}

func TestParseLeadingBackslashLiteral(t *testing.T) {
	src := `rule x
  forms: default
  back\slash
`
	g, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	tpl := g.rules["x"].Alternatives[0].Forms["default"]
	lit, ok := tpl[0].(Literal)
	if !ok || lit.Text != `back\slash` {
		t.Errorf("text = %q, tok = %T", func() string {
			if l, ok := tpl[0].(Literal); ok {
				return l.Text
			}
			return ""
		}(), tpl[0])
	}
}

// Parse-error tests.

func TestParseErrorWeightZero(t *testing.T) {
	src := "rule x\n  forms: default\n  weight=0 a\n"
	_, err := Parse(src)
	if err == nil {
		t.Fatal("expected error for weight=0")
	}
	if !strings.Contains(err.Error(), "weight") {
		t.Errorf("error %q should mention weight", err)
	}
	if !strings.Contains(err.Error(), "3") {
		t.Errorf("error %q should include line number 3", err)
	}
}

func TestParseErrorDefaultFormHasDefaultTemplate(t *testing.T) {
	src := "rule x\n  forms: default={}s\n  a\n"
	_, err := Parse(src)
	if err == nil {
		t.Fatal("expected error for default form having default template")
	}
}

func TestParseErrorEmptyBracesOutsideFormDefault(t *testing.T) {
	src := "rule x\n  forms: default\n  hi {}\n"
	_, err := Parse(src)
	if err == nil {
		t.Fatal("expected error for {} in entry template")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "self") &&
		!strings.Contains(err.Error(), "{}") {
		t.Errorf("error %q should reference empty/self braces", err)
	}
}

func TestValidateUnknownFormNamesIt(t *testing.T) {
	src := "rule x\n  forms: default\n  hi\n\nrule y\n  forms: default\n  {x:plural}\n"
	g, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	err = g.Validate()
	if err == nil {
		t.Fatal("expected error for unknown form")
	}
	if !strings.Contains(err.Error(), "plural") {
		t.Errorf("error %q should name unknown form", err)
	}
}

func TestParseErrorLowercaseSaveName(t *testing.T) {
	src := "rule x\n  forms: default\n  hi\n\nrule y\n  forms: default\n  {x as foo}\n"
	_, err := Parse(src)
	if err == nil {
		t.Fatal("expected error for lowercase save name")
	}
}

func TestParseErrorUppercaseRuleName(t *testing.T) {
	src := "rule X\n  forms: default\n  hi\n"
	_, err := Parse(src)
	if err == nil {
		t.Fatal("expected error for uppercase rule name")
	}
}

func TestParseErrorMissingDefaultForm(t *testing.T) {
	src := `rule animal
  forms: default, plural={}s
  | mice
`
	_, err := Parse(src)
	if err == nil {
		t.Fatal("expected error for missing default form value")
	}
}

func TestParseErrorTooManyForms(t *testing.T) {
	src := `rule animal
  forms: default, plural={}s
  cat | cats | extra
`
	_, err := Parse(src)
	if err == nil {
		t.Fatal("expected error for extra pipe-separated value")
	}
}

func TestParseErrorMissingRuleName(t *testing.T) {
	src := "rule\n  forms: default\n  hi\n"
	_, err := Parse(src)
	if err == nil {
		t.Fatal("expected error for missing rule name")
	}
}

// A rule with no forms: line is treated as if it declared `forms: default`.
// The parsed shape must match the explicit-default form exactly.
func TestParseImplicitDefaultFormSingleEntry(t *testing.T) {
	implicit := "rule greeting\n  hello\n"
	explicit := "rule greeting\n  forms: default\n  hello\n"
	gImp, err := Parse(implicit)
	if err != nil {
		t.Fatalf("Parse(implicit): %v", err)
	}
	gExp, err := Parse(explicit)
	if err != nil {
		t.Fatalf("Parse(explicit): %v", err)
	}
	if !reflect.DeepEqual(gImp.rules, gExp.rules) {
		t.Errorf("implicit and explicit forms produce different rules:\n  imp=%#v\n  exp=%#v", gImp.rules, gExp.rules)
	}
}

func TestParseImplicitDefaultFormMultiEntry(t *testing.T) {
	src := "rule greeting\n  hello\n  hi\n  weight=2 hey\n"
	g, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	r := g.rules["greeting"]
	if r == nil {
		t.Fatal("rule greeting missing")
	}
	if len(r.Forms) != 1 || r.Forms[0].Name != "default" {
		t.Errorf("forms = %+v, want single default", r.Forms)
	}
	if len(r.Alternatives) != 3 {
		t.Fatalf("alternatives = %d, want 3", len(r.Alternatives))
	}
	wantText := []string{"hello", "hi", "hey"}
	wantWeight := []uint{1, 1, 2}
	for i := range wantText {
		lit, ok := r.Alternatives[i].Forms["default"][0].(Literal)
		if !ok || lit.Text != wantText[i] {
			t.Errorf("alt %d text = %#v, want %q", i, r.Alternatives[i].Forms["default"][0], wantText[i])
		}
		if r.Alternatives[i].Weight != wantWeight[i] {
			t.Errorf("alt %d weight = %d, want %d", i, r.Alternatives[i].Weight, wantWeight[i])
		}
	}
}

// A `forms:` line that follows an entry inside the same rule is a parse
// error: the rule has already been treated as having an implicit
// `forms: default` once the first entry was consumed.
func TestParseErrorFormsAfterEntry(t *testing.T) {
	src := "rule x\n  hi\n  forms: default\n"
	_, err := Parse(src)
	if err == nil {
		t.Fatal("expected error when forms: follows an entry")
	}
	if !strings.Contains(err.Error(), "forms") {
		t.Errorf("error %q should mention forms:", err)
	}
}

func TestParseErrorDuplicateRule(t *testing.T) {
	src := `rule x
  forms: default
  a

rule x
  forms: default
  b
`
	_, err := Parse(src)
	if err == nil {
		t.Fatal("expected error for duplicate rule")
	}
}

func TestParseErrorUnclosedBrace(t *testing.T) {
	src := "rule x\n  forms: default\n  {oops\n"
	_, err := Parse(src)
	if err == nil {
		t.Fatal("expected error for unclosed brace")
	}
}

func TestParseErrorIncludesLineColumn(t *testing.T) {
	src := "rule x\n  forms: default\n  weight=0 a\n"
	_, err := Parse(src)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	// line 3, column should be present
	if !strings.Contains(msg, "3:") && !strings.Contains(msg, "line 3") {
		t.Errorf("error %q should include line 3 with column", msg)
	}
}

// B4: Whitespace immediately after '=' in a forms declaration must not
// leak into the default template as a literal space. Both forms should
// produce the same parsed template.
func TestParseFormsDefaultTrimsLeadingWhitespaceAfterEquals(t *testing.T) {
	with := "rule animal\n  forms: default, plural = {}s\n  cat\n"
	without := "rule animal\n  forms: default, plural={}s\n  cat\n"
	gWith, err := Parse(with)
	if err != nil {
		t.Fatalf("Parse(with): %v", err)
	}
	gWithout, err := Parse(without)
	if err != nil {
		t.Fatalf("Parse(without): %v", err)
	}
	a, b := gWith.rules["animal"].Forms[1].Default, gWithout.rules["animal"].Forms[1].Default
	if !reflect.DeepEqual(a, b) {
		t.Errorf("templates differ:\n with    = %#v\n without = %#v", a, b)
	}
	want := Template{SelfRef{}, Literal{Text: "s"}}
	if !reflect.DeepEqual(a, want) {
		t.Errorf("with-space template = %#v, want %#v", a, want)
	}
}

// B4 corollary: whitespace internal to the template body — i.e. between
// non-space tokens — is preserved. (Trailing whitespace at end-of-line
// is consumed by line-trimming before the template body is reached, so
// the meaningful preservation test is for internal whitespace.)
func TestParseFormsDefaultPreservesInternalWhitespace(t *testing.T) {
	src := "rule x\n  forms: default, suffixed= pre {} post\n  a\n"
	g, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := Template{Literal{Text: "pre "}, SelfRef{}, Literal{Text: " post"}}
	got := g.rules["x"].Forms[1].Default
	if !reflect.DeepEqual(got, want) {
		t.Errorf("template = %#v, want %#v", got, want)
	}
}

// B5: middle-form omission ("a | | c" on a 3-form rule) is rejected.
func TestParseErrorMiddleFormOmission(t *testing.T) {
	src := `rule x
  forms: default, plural={}s, past={}ed
  cat | | catted
`
	_, err := Parse(src)
	if err == nil {
		t.Fatal("expected error for middle-form omission")
	}
	if !strings.Contains(err.Error(), "middle") && !strings.Contains(err.Error(), "omit") {
		t.Errorf("error %q should explain middle-form omission", err)
	}
}

// B5 negative case: trailing-only omission remains legal.
func TestParseTrailingFormOmissionAllowed(t *testing.T) {
	src := `rule x
  forms: default, plural={}s
  cat |
`
	if _, err := Parse(src); err != nil {
		t.Fatalf("trailing omission should parse: %v", err)
	}
}

// B3: column for a duplicate-form error must point to the *second*
// occurrence, not the first.
func TestParseErrorDuplicateFormColumn(t *testing.T) {
	src := "rule x\n  forms: a, b, a\n  q\n"
	_, err := Parse(src)
	if err == nil {
		t.Fatal("expected duplicate-form error")
	}
	pe, ok := err.(*ParseError)
	if !ok {
		t.Fatalf("err is %T, want *ParseError", err)
	}
	if pe.Line != 2 {
		t.Errorf("line = %d, want 2", pe.Line)
	}
	// "  forms: a, b, a"
	//  0123456789012345
	// The second 'a' is at byte offset 15, 1-based column 16. The bug
	// was that the parser used strings.Index of "a" in the line, which
	// returned the *first* occurrence (column 10).
	if pe.Col != 16 {
		t.Errorf("col = %d, want 16 (pointing at the second 'a')", pe.Col)
	}
}

// B7: literal '\|' is two literal characters (backslash, pipe). The pipe
// still splits because '\|' is not a recognised escape.
func TestParseBackslashPipeIsLiteralAndPipeStillSplits(t *testing.T) {
	src := "rule x\n  forms: default, plural={}s\n  a\\|b | c\n"
	g, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	tpl := g.rules["x"].Alternatives[0].Forms["default"]
	want := Template{Literal{Text: "a\\|b"}}
	if !reflect.DeepEqual(tpl, want) {
		t.Errorf("default = %#v, want %#v", tpl, want)
	}
	plural := g.rules["x"].Alternatives[0].Forms["plural"]
	wantPlural := Template{Literal{Text: "c"}}
	if !reflect.DeepEqual(plural, wantPlural) {
		t.Errorf("plural = %#v, want %#v", plural, wantPlural)
	}
}

// B7: '\\' is the escape for a single literal backslash.
func TestParseDoubleBackslashIsSingleLiteralBackslash(t *testing.T) {
	src := "rule x\n  forms: default\n  a\\\\b\n"
	g, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	tpl := g.rules["x"].Alternatives[0].Forms["default"]
	want := Template{Literal{Text: "a\\b"}}
	if !reflect.DeepEqual(tpl, want) {
		t.Errorf("template = %#v, want %#v", tpl, want)
	}
}

// B7: '\n' is *not* an escape — it's a literal backslash followed by 'n'.
func TestParseBackslashNIsLiteralBackslashThenN(t *testing.T) {
	src := "rule x\n  forms: default\n  a\\nb\n"
	g, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	tpl := g.rules["x"].Alternatives[0].Forms["default"]
	want := Template{Literal{Text: "a\\nb"}}
	if !reflect.DeepEqual(tpl, want) {
		t.Errorf("template = %#v, want %#v", tpl, want)
	}
}

// B8: ErrDuplicateRule is returned via errors.Is.
func TestParseDuplicateRuleWrapsSentinel(t *testing.T) {
	src := "rule x\n  forms: default\n  a\n\nrule x\n  forms: default\n  b\n"
	_, err := Parse(src)
	if !errors.Is(err, ErrDuplicateRule) {
		t.Fatalf("err = %v; want errors.Is ErrDuplicateRule", err)
	}
}

// B8: Parse no longer rejects an undefined rule reference; Validate does.
// Parse must succeed so callers can Merge multiple split-source files
// before any cross-file references are checked.
func TestParseUndefinedRuleWrapsSentinel(t *testing.T) {
	src := "rule x\n  forms: default\n  {missing}\n"
	g, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if err := g.Validate(); !errors.Is(err, ErrUndefinedRule) {
		t.Fatalf("Validate err = %v; want errors.Is ErrUndefinedRule", err)
	}
}

// B8: Same as above for an unknown-form reference.
func TestParseUnknownFormWrapsSentinel(t *testing.T) {
	src := "rule x\n  forms: default\n  hi\n\nrule y\n  forms: default\n  {x:plural}\n"
	g, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if err := g.Validate(); !errors.Is(err, ErrUnknownForm) {
		t.Fatalf("Validate err = %v; want errors.Is ErrUnknownForm", err)
	}
}

// Validate succeeds on a fully-resolved grammar.
func TestValidateAllReferencesResolved(t *testing.T) {
	src := `rule a
  forms: default, plural={}s
  cat

rule x
  forms: default
  {a} {a:plural}
`
	g, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if err := g.Validate(); err != nil {
		t.Errorf("Validate: %v", err)
	}
}

// The split-source case: each file Parses on its own; the cross-file
// reference resolves once they have been merged into one grammar.
func TestParseMergeValidateSplitSource(t *testing.T) {
	parts := "rule color\n  red\n  blue\n"
	top := "rule sentence\n  the {color} thing\n"
	gParts, err := Parse(parts)
	if err != nil {
		t.Fatalf("Parse(parts): %v", err)
	}
	gTop, err := Parse(top)
	if err != nil {
		t.Fatalf("Parse(top): %v", err)
	}
	if err := gTop.Merge(gParts); err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if err := gTop.Validate(); err != nil {
		t.Errorf("Validate after merge: %v", err)
	}
}

// T6: CRLF line endings should produce the same Grammar as LF.
func TestParseCRLF(t *testing.T) {
	src := `rule animal
  forms: default, plural={}s
  cat
  mouse | mice

rule x
  forms: default
  hi {animal:plural}
`
	gLF, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse(LF): %v", err)
	}
	gCRLF, err := Parse(strings.ReplaceAll(src, "\n", "\r\n"))
	if err != nil {
		t.Fatalf("Parse(CRLF): %v", err)
	}
	if !reflect.DeepEqual(gLF.rules, gCRLF.rules) {
		t.Errorf("CRLF parsed differently:\n  LF=%#v\n  CRLF=%#v", gLF.rules, gCRLF.rules)
	}
}

// Q17: Generate against an empty grammar must error.
func TestGenerateOnEmptyGrammarErrors(t *testing.T) {
	g, err := Parse("")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	_, err = g.Generate("anything", newRand(1))
	if !errors.Is(err, ErrUndefinedRule) {
		t.Fatalf("err = %v; want errors.Is ErrUndefinedRule", err)
	}
}

// Round-trip: parse then generate.
func TestParseAndGenerateRoundTrip(t *testing.T) {
	src := `rule animal
  forms: default, plural={}s
  cat
  mouse | mice

rule story
  forms: default
  the {animal as A} met another {*A}
  two {animal:plural}
`
	g, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	for i := range 30 {
		out, err := g.Generate("story", newRand(int64(i)))
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		if !strings.HasPrefix(out, "the ") && !strings.HasPrefix(out, "two ") {
			t.Errorf("unexpected: %q", out)
		}
		if rest, ok := strings.CutPrefix(out, "two "); ok {
			if rest != "cats" && rest != "mice" {
				t.Errorf("plural unexpected: %q", out)
			}
		}
	}
}

func TestParseEntryTags(t *testing.T) {
	src := `rule snack
  forms: default, plural={}s
  apple | apples tags=fruit,food
`
	g, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	alts := g.rules["snack"].Alternatives
	if len(alts) != 1 {
		t.Fatalf("alternatives = %d, want 1", len(alts))
	}
	if got, want := alts[0].Tags, []string{"fruit", "food"}; !slices.Equal(got, want) {
		t.Fatalf("Tags = %#v, want %#v", got, want)
	}
	defaultTpl := alts[0].Forms["default"]
	if got, want := defaultTpl, (Template{Literal{Text: "apple"}}); !templatesEqual(got, want) {
		t.Fatalf("default template = %#v, want %#v", got, want)
	}
	pluralTpl := alts[0].Forms["plural"]
	if got, want := pluralTpl, (Template{Literal{Text: "apples"}}); !templatesEqual(got, want) {
		t.Fatalf("plural template = %#v, want %#v", got, want)
	}
}

func TestParseEntryTagsBeforeComment(t *testing.T) {
	src := `rule snack
  apple tags=fruit # available only when fruit is present
`
	g, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got, want := g.rules["snack"].Alternatives[0].Tags, []string{"fruit"}; !slices.Equal(got, want) {
		t.Fatalf("Tags = %#v, want %#v", got, want)
	}
}

func TestParseErrorEmptyEntryTags(t *testing.T) {
	src := `rule snack
  apple tags=
`
	_, err := Parse(src)
	if err == nil {
		t.Fatal("expected error for empty tags")
	}
	if !strings.Contains(err.Error(), "tags") {
		t.Fatalf("err = %v, want tags", err)
	}
}

func TestParseErrorInvalidEntryTag(t *testing.T) {
	src := `rule snack
  apple tags=Fruit
`
	_, err := Parse(src)
	if err == nil {
		t.Fatal("expected error for invalid tag")
	}
	if !strings.Contains(err.Error(), "invalid tag") {
		t.Fatalf("err = %v, want invalid tag", err)
	}
}

func TestParseTagsInsideReferenceAreNotTrailingTags(t *testing.T) {
	src := `rule snack
  {tags=fruit}
`
	_, err := Parse(src)
	if err == nil {
		t.Fatal("expected reference parse error")
	}
	if strings.Contains(err.Error(), "tag") && !strings.Contains(err.Error(), "rule name") {
		t.Fatalf("err = %v, want reference parsing error rather than tag parsing", err)
	}
}
