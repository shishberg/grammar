# Grammar source format: formal grammar

The grammar source format itself, in EBNF (ISO/IEC 14977 conventions:
`,` for concatenation, `|` for alternation, `[ ... ]` for optional,
`{ ... }` for zero-or-more, `(* ... *)` for comments, `?` for
informally-defined character classes, terminals in double quotes).

This is the authoritative shape of `Parse(source)` input. The prose
spec in [design.md](design.md) explains semantics and trade-offs; this
document is just the syntax.

## Top level

```ebnf
source       = { line } ;
line         = [ content ] [ comment ] newline ;
content      = blank
             | rule-header
             | forms-line
             | entry-line ;

blank        = { wsp } ;
rule-header  = { wsp } , "rule" , wsp+ , rule-name , { wsp } ;
forms-line   = { wsp } , "forms:" , { wsp } , form-spec , { { wsp } , "," , { wsp } , form-spec } , { wsp } ;
entry-line   = { wsp } , [ weight-tag , wsp+ ] , entry-body , [ wsp+ , tags-tag ] , { wsp } ;
```

The parser is line-oriented:

- `rule-header` opens a new rule. The first non-blank line that
  follows MAY be a `forms-line`; if it is, the rule has the declared
  forms. If the first non-blank line after `rule-header` is an
  `entry-line` instead, the rule is treated as if it had declared
  `forms: default` (a single form named `default`, no inflection).
- Once an `entry-line` has been consumed, a `forms-line` for the
  same rule is a parse error.
- Subsequent non-blank lines are `entry-line`s for the rule, until
  the next blank line or the next `rule-header`.
- A line consisting only of `blank` (possibly with a `comment`) is a
  blank line; blank lines separate rules and are otherwise ignored.

## Forms

```ebnf
form-spec    = form-name , [ { wsp } , "=" , { wsp } , template ] ;
weight-tag   = "weight" , "=" , digit , { digit } ;
tags-tag     = "tags" , "=" , rule-name , { "," , rule-name } ;
```

Constraints not expressible in EBNF:

- The first `form-spec` in a `forms-line` MUST NOT supply a default
  template (`= template`); it is the rule's *default form*, and a
  default template would be self-referential.
- `weight-tag`'s digits MUST denote an integer ≥ 1. `weight=0` is
  rejected.
- The `weight=` prefix is recognised only at the start of an
  `entry-line` (after leading `wsp`). A literal `weight=` later in the
  line is template text.
- A trailing `tags=` declaration marks prerequisites for the entry.
  Each tag uses the same lowercase identifier shape as `rule-name`.

## Entries

```ebnf
entry-body   = form-value , { "|" , form-value } ;
form-value   = { wsp } , template , { wsp } ;
```

Constraints not expressible in EBNF:

- The number of `form-value`s in an entry MUST NOT exceed the number
  of `form-spec`s declared by the rule's `forms-line`.
- The first `form-value` (the default-form value) MUST be supplied by
  every entry; it cannot be omitted or left empty.
- Trailing `form-value`s MAY be omitted; missing values fall back to
  the corresponding `form-spec`'s default template.
- *Middle*-form omission — an empty `form-value` followed by any
  non-empty later `form-value` — is rejected. Supply the middle value
  explicitly.
- Leading and trailing whitespace inside each `form-value` is trimmed;
  whitespace inside the template body is preserved.

## Templates

```ebnf
template     = { template-atom } ;
template-atom = literal-char | escape | reference ;

literal-char = ? any single byte that is not "{", "}", "|", "#",
                 "\", or a newline byte ? ;

escape       = "\{" | "\}" | "\#" | "\\" ;

reference    = "{" , ref-body , "}" ;
ref-body     = self-ref | rule-ref | recall ;
self-ref     = (* empty *) ;
rule-ref     = rule-name , [ ":" , form-name ] , [ wsp+ , "as" , wsp+ , save-name ] ;
recall       = "*" , save-name ;
```

Constraints not expressible in EBNF:

- `self-ref` (the empty `{}`) is legal *only* inside a non-default
  form's default template (the `template` that follows `=` in a
  `form-spec`). It is a parse error anywhere else.
- `rule-ref`'s `form-name` MUST name a form declared on the referenced
  rule. (Checked by `Validate` once the full grammar is assembled, or
  at generation time if `Validate` was skipped.)
- A backslash followed by any byte *other* than `{`, `}`, `#`, `\` is
  *not* an escape: the backslash and the following byte are both
  literal. A trailing backslash at end of line is also literal.

### Pipe-splitting interaction

`entry-body` is split on `|` *before* templates are parsed. A backslash
suppresses splitting on the byte that follows it, even when that byte
isn't otherwise an escapable character. So `a\|b` is one
form-value whose literal text is `a\|b` (the `\|` is preserved
verbatim — it's not a recognised escape, but the `|` does not split).
Pipes inside `{...}` references also do not split.

## Comments and whitespace

```ebnf
comment      = "#" , { ? any byte that is not a newline byte ? } ;
wsp          = " " | tab ;
tab          = ? horizontal tab ? ;
newline      = "\n" | "\r\n" ;
```

- A `#` outside a `reference` and not preceded by a backslash starts a
  comment; the `#` and everything after it on the line is discarded.
- `\#` decodes to a literal `#`.
- A `#` inside a `reference` is illegal (rule names, form names, and
  save names do not contain `#`); the parser will report a parse
  error before it would treat the `#` as a comment.
- The parser strips a single trailing `\r` from each line before
  processing, so CRLF-terminated source produces the same grammar as
  LF-terminated source.

## Identifiers

```ebnf
rule-name    = lower , { lower | digit | "_" } ;
form-name    = lower , { lower | digit | "_" } ;
save-name    = upper , { upper | digit | "_" } ;

lower        = "a" | "b" | "c" | "d" | "e" | "f" | "g"
             | "h" | "i" | "j" | "k" | "l" | "m" | "n"
             | "o" | "p" | "q" | "r" | "s" | "t" | "u"
             | "v" | "w" | "x" | "y" | "z" ;
upper        = "A" | "B" | "C" | "D" | "E" | "F" | "G"
             | "H" | "I" | "J" | "K" | "L" | "M" | "N"
             | "O" | "P" | "Q" | "R" | "S" | "T" | "U"
             | "V" | "W" | "X" | "Y" | "Z" ;
digit        = "0" | "1" | "2" | "3" | "4" | "5" | "6"
             | "7" | "8" | "9" ;
```

Rule and form names share the lowercase ASCII space; save names live
in the uppercase ASCII space. The case split makes the two namespaces
visually distinct in templates without depending on syntactic position
to disambiguate.

## What this grammar does NOT cover

- *Rule shape* invariants (a rule must declare a `forms-line` before
  its first entry, every entry must supply the default form, each
  rule must have at least one entry). These are enforced by the
  parser and listed in [design.md](design.md) but live above the
  per-line grammar.
- *Encoding*. The format is byte-oriented; literal bytes outside the
  ASCII control set pass through unmodified. UTF-8 input works
  because the special bytes (`{`, `}`, `|`, `#`, `\`, `\n`, `\r`) are
  all single-byte ASCII and can't appear inside a UTF-8 multibyte
  sequence.
- *Rule reference target validity* (the named rule must exist; the
  form must be declared). These are post-parse checks against the
  full rule set, and on the generation path against the recursion
  limit.
