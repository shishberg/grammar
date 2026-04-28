# Grammar package: design principles

Starting principles for a Go package that parses and generates from a
rule-based text grammar.

## 1. Formal spec

The source format has a written grammar (EBNF or similar). Edge cases
are defined, not "whatever the parser does." A grammar engine is no
better than the spec it's written against — without one, every quirk
becomes load-bearing.

## 2. Library API first; source format is a thin layer on top

The package operates on parsed grammars (a tree of typed values). A
text source format is one way grammars arrive; programmatic
construction is another. How grammars get into the parser — one file,
many files, embedded blob, fetched over the network — is a host
concern outside the package.

## 3. Small core, extension points

Built-in: rule references, weighted alternatives, inflections,
variable save/recall.

Pluggable: post-processors. English `a` → `an` cleanup ships as one
optional post-processor; underscore → space ships as another;
non-English hosts can leave them off and add their own.

## 4. One syntactic form per concept

Each concept gets exactly one surface form. No "this character does
X here, but Y there"; no parallel ways to spell the same thing. The
parser's job is to recognise the form, not to disambiguate between
several.

## 5. Explicit beats clever

Weights look like an explicit tag (e.g. `weight=4 …`), not a leading
character that has to be detected. No "if the line starts with digits,
it's a probability." Surface forms whose meaning depends on subtle
positional cues are hard to spec, hard to write, and hard to read.

## 6. Inflections live with their data

`mouse|mice` on the same line as `mouse`. A modifier-based "just
append s" approach is wrong for mice/oxen and forces the host to
sprinkle exceptions through the grammar; keeping irregulars next to
the singular form is the only sane place for them.

## 7. Deterministic when seeded

Take a `*rand.Rand` (or a seed) so output is reproducible — useful for
tests and "share this generation" features. A globally-seeded RNG
makes tests painful and outputs unshareable.

## Considered and dropped

- **"No state — be a real CFG."** Tempting for theoretical cleanliness,
  but variable save/recall is one of the most useful features and
  principle-driven austerity isn't worth losing it. The package is
  explicitly *not* a pure CFG; we document what context we keep.
- **"Files are the source of truth."** Conflates with deployment.
  Left as a host concern per principle 2.
- **"Auto-cleanup is core."** It's English policy. Belongs as a
  pluggable post-processor (principle 3), not in the engine.
