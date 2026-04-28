# Grammar package: design

A Go package that parses and generates from a rule-based text grammar.

Principles for this package are in [principles.md](principles.md). The
formal source-format grammar is in [ebnf.md](ebnf.md). This document
specifies the conceptual model, source format, and public API.

## Conceptual model

A grammar is a set of named rules. Each rule declares one or more
inflectional *forms* (e.g. default, plural, past) and a list of
weighted *alternatives*. Each alternative is a per-form *template*; a
template is a sequence of literal text and substitution tokens.

```go
package grammar

type Grammar struct {
    rules map[string]*Rule
}

type Rule struct {
    Forms        []FormSpec     // index 0 is the default form
    Alternatives []Alternative
}

type FormSpec struct {
    Name    string    // "default", "plural", "past", ...
    Default Template  // used when an entry doesn't override this form
}

type Alternative struct {
    Weight uint                  // default 1
    Forms  map[string]Template   // keyed by form name
}

type Template []Token

type Token interface{ token() }
type Literal struct{ Text string }
type RuleRef struct {
    Rule string  // name of the rule to expand
    Form string  // "" = default form
    Save string  // "" = don't save; uppercase name = save under that name
}
type Recall struct{ Name string }
type SelfRef struct{}            // only legal inside a FormSpec.Default
```

Generation walks alternatives picking by weight, then walks the chosen
template. `RuleRef` recurses; `Recall` looks up a previously-saved
expansion in scope.

The package is explicitly *not* a pure context-free grammar:
`{rule as NAME}` and `{*NAME}` introduce context. Saved variables live
for the duration of one top-level `Generate` call.

## Source format

### Tour

```
rule noun
  forms: default, plural={}s
  mouse | mice
  fox   | foxes
  dog            # plural defaults to "dogs"
  weight=2 cat   # cat occurs 2x as often

rule adjective
  forms: default
  golden
  sleeping

rule tavern
  forms: default
  The {adjective} {noun}
  The {adjective} {noun as ANIMAL} returns to its {*ANIMAL} home
  Two {noun:plural}
```

### Rules

- A rule begins with `rule NAME` on its own line. Rules are
  blank-line separated.
- The first non-blank line after `rule NAME` may be a `forms:` line
  declaring the rule's inflectional scheme. If it is omitted, the
  rule is treated as if it had declared `forms: default` — a single
  form named `default`, no inflection.
- Subsequent non-blank lines are *entries* (one alternative each)
  until the next blank line or `rule` keyword.
- A `forms:` line that follows any entry of the same rule is a parse
  error.

### Forms declaration

```
forms: NAME [= TEMPLATE] { , NAME [= TEMPLATE] }
```

The `forms:` line is optional. Omitting it is shorthand for
`forms: default` — a single form named `default` with no default
template. Single-form rules don't need to write the line.

The first listed form is the rule's *default form*. A form other than
the default may give a *default template* used when an entry omits
that form's value. Inside a default template, `{}` (the *self-ref*)
means "this entry's default-form value." `{}` is the only legal use of
empty braces, and it is only legal inside a non-default form's default
template.

The default form itself must not have a default template (it would be
self-referential, and there's no fallback source for its value). Every
entry must supply a value for the default form.

Whitespace around `=` is optional. Leading whitespace after the `=`
(between the `=` and the start of the template body) is trimmed.
Whitespace inside the template body — including between tokens — is
preserved, so `forms: default, plural= {}s` and
`forms: default, plural={}s` produce the same template.

Examples:

- `forms: default` — single form, no inflection.
- `forms: default, plural={}s` — pluralize-by-default-rule, override
  per entry as needed.
- `forms: default, comparative={}er, superlative={}est` — adjective
  inflection.

### Entries

An entry is one alternative for a rule:

```
[weight=N] TEMPLATE { | TEMPLATE }
```

- Pipe-separated form values appear in the order declared by `forms:`.
  Each form value has its leading and trailing whitespace trimmed.
- Trailing forms may be omitted; missing values are filled in by that
  form's default template. Middle-form omission is not supported in
  v1: an empty pipe-separated value followed by any non-empty later
  value (e.g. `a | | c` in a 3-form rule) is a parse error. Supply the
  middle value explicitly if you want to override one in the middle.
- The default form must always be supplied (per *Forms declaration*).
- `weight=N` (integer ≥ 1) is a per-line tag setting the
  alternative's weight. Default 1. The `weight=` tag is recognised
  only as a prefix at the start of an entry line; a literal `weight=`
  appearing later in the line is template text.

### Templates

A template is a sequence of literal characters and `{...}` references.

```
ref       = "{" ref-body "}"
ref-body  = self-ref | rule-ref | recall
self-ref  = (empty)              // legal only in a form-default template
rule-ref  = NAME [":" FORM-NAME] [ws "as" ws SAVE-NAME]
recall    = "*" SAVE-NAME
```

- `{rule}` expands the named rule's default form.
- `{rule:form}` expands a specific form. Referencing a form the rule
  doesn't declare is a parse error.
- `{rule as NAME}` expands and saves the result under `NAME` for
  later recall in the same generation.
- `{rule:form as NAME}` combines the two.
- `{*NAME}` recalls the saved value of `NAME`.

Identifier rules:

- `NAME` (rule name) and `FORM-NAME`: lowercase ASCII letter, followed
  by lowercase ASCII letters, digits, or underscore. (`[a-z][a-z0-9_]*`)
- `SAVE-NAME`: uppercase ASCII letter, followed by uppercase ASCII
  letters, digits, or underscore. (`[A-Z][A-Z0-9_]*`)

The case split keeps the two namespaces visually distinct in template
text, even though the parser disambiguates them by syntactic position.

The whitespace around `as` (between the form-or-rule name and the
save-name) must be at least one ASCII space or tab.

The recognised backslash escapes are `\{`, `\}`, `\#`, and `\\`. They
decode to a single `{`, `}`, `#`, or `\` respectively. Any other `\X`
sequence is two literal characters: a backslash followed by `X`
evaluated normally. A trailing backslash at end of line is literal.

The escape `\` does, however, suppress pipe-splitting on the byte that
follows it during entry-line splitting: `a\|b` is one pipe-separated
value whose literal text is `a\|b` (backslash-pipe-b — neither byte is
decoded away because `\|` is not in the escape list, but the `|` does
not split the entry). To include an actual pipe character with no
backslash adjacent, place it inside `{...}` (where pipes do not split)
or rely on the backslash-protect behaviour above.

### Comments and whitespace

- `#` starts a comment that runs to end of line. `#` inside a `{...}`
  reference or escaped (`\#`) is literal.
- Leading indentation on entry/forms lines is permitted and ignored —
  it's purely for readability.
- Blank lines separate rules and are otherwise ignored. A line
  containing only whitespace counts as a blank line.
- Lines may be terminated by `\n` or `\r\n`. The parser strips a
  trailing `\r` if present, so CRLF-terminated source produces the
  same grammar as LF-terminated source.

## Public API

```go
package grammar

// Parse builds a Grammar from a source-format string. Parse only
// checks syntax inside the source it sees; cross-source rule
// references are resolved by Validate after merging.
// Multiple Parse calls can be merged with Grammar.Merge.
func Parse(source string) (*Grammar, error)

// NewGrammar returns an empty Grammar that callers can populate via AddRule.
func NewGrammar() *Grammar

// AddRule installs r under name. Returns an error if name is already
// defined or if r fails the same shape checks Parse applies.
func (g *Grammar) AddRule(name string, r *Rule) error

// Validate checks that every {rule[:form]} reference in g resolves to
// a defined rule (and declared form). Call after Merge or AddRule and
// before Generate to surface dangling references up front. Errors
// wrap ErrUndefinedRule or ErrUnknownForm.
func (g *Grammar) Validate() error

// Generate produces one expansion of the named rule's default form.
func (g *Grammar) Generate(rule string, rng *rand.Rand) (string, error)

// GenerateWith applies post-processors in order to the generated string.
func (g *Grammar) GenerateWith(
    rule string, rng *rand.Rand, post ...PostProcessor,
) (string, error)

// Merge adds the rules of other into g. When both grammars define a
// rule with the same name and matching form schemes, their
// alternatives are combined (g's first, then other's). Mismatched
// form schemes wrap ErrFormSchemeMismatch. Merge(nil) is a no-op.
func (g *Grammar) Merge(other *Grammar) error

// PostProcessor transforms generated output. Pluggable per principle 3.
type PostProcessor func(string) string
```

Programmatic construction goes through `NewGrammar` plus `AddRule`,
populating the exported `Rule`, `FormSpec`, `Alternative`, and
`Template` types directly. `AddRule` validates rule shape (default form
present, default form has no default template, every alternative
supplies the default form) so programmatic and parsed grammars satisfy
the same invariants.

Errors from generation and parsing wrap a small set of sentinel values
that callers can match with `errors.Is`: `ErrUndefinedRule`,
`ErrUnknownForm`, `ErrUnsavedRecall`, `ErrRecursionLimit`,
`ErrDuplicateRule`, and `ErrFormSchemeMismatch`.

`Merge` combines two same-name rules whose form schemes agree (same
form names in the same order, with structurally equal form-default
templates); the combined rule has g's alternatives followed by
other's, in source order, weights preserved. A mismatched form
scheme wraps `ErrFormSchemeMismatch`. `AddRule` does not combine —
adding a name that's already present returns `ErrDuplicateRule`.

### Built-in post-processors

Language-specific helpers live in subpackages so the core package has
no language opinions:

- `grammar/postproc/english.AAn` — converts `a` to `an` before
  vowel-sounding words.
- `grammar/postproc/english.UnderscoreToSpace` — substitutes spaces
  for underscores (used to glue compound words past inflection
  boundaries).

Hosts opt in by passing them to `GenerateWith`.

## Examples

### A minimal generator

```
rule greeting
  forms: default
  hello
  hi
  hey
```

```go
g, _ := grammar.Parse(src)
out, _ := g.Generate("greeting", rng)  // "hi", "hey", "hello"
```

### Inflection and back-reference

```
rule animal
  forms: default, plural={}s
  cat
  dog
  mouse | mice
  ox    | oxen

rule story
  forms: default
  The {animal as A} chased the other {*A}.
  Two {animal:plural} walked into a bar.
```

`{*A}` recalls the same animal generated earlier in the sentence. The
plural form of `mouse` is `mice` because the entry overrides the
form-default template.

## Out of scope for v1

- Conditional / probabilistic alternatives beyond per-line weights —
  e.g. picking a different alternative depending on what came before.
  Defer until a real grammar wants it.
- `@include` or any cross-rule import semantics. The host parses each
  file with `Parse`, combines them with `Merge`, and calls `Validate`
  on the assembled grammar. The library doesn't know about files.
- Built-in helpers like random integers, year picks, set
  permutations. If they appear later, they get modelled as pluggable
  template functions, not new directives.
- Multi-form variable saving. `{noun as X}` saves the *expansion*; you
  cannot recall a saved noun in a different inflection. Revisit if
  this bites.

## Open questions

- **Recursion-limit knob.** The generator currently uses a hard-coded
  depth cap. If a host needs to tune it, expose a `Grammar.SetMaxDepth`
  or a `GenerateOption` — deferred until a real caller asks.
