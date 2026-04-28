# Grammar package: design principles

Starting principles for a Go package that parses and generates from a
rule-based text grammar, inspired by `mezzacotta-generator` but not
aiming for compatibility with it.

## 1. Formal spec

The source format has a written grammar (EBNF or similar). Edge cases
are defined, not "whatever the parser does." Mezzacotta's biggest
weakness is having no spec — you reverse-engineer it from PHP source.
We don't repeat that.

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

Mezzacotta has `_S` substitution suffixes and `|S` format declarations
and leading-digit probability and `>` else syntax. Each concept gets
exactly one surface form here.

## 5. Explicit beats clever

Weights look like an explicit tag (e.g. `weight=4 …`), not a leading
character that has to be detected. No "if the line starts with digits,
it's a probability." Sigil-juggling is what makes mezzacotta hard to
spec.

## 6. Inflections live with their data

`mouse|mice` on the same line as `mouse`. Tracery's `.s` modifier
(just-append-s) is wrong for mice/oxen; mezzacotta got this right and
it's worth keeping.

## 7. Deterministic when seeded

Take a `*rand.Rand` (or a seed) so output is reproducible — useful for
tests and "share this generation" features. Mezzacotta uses Python's
global random state, which makes tests painful.

## Considered and dropped

- **"No state — be a real CFG."** Tempting for theoretical cleanliness,
  but variable save/recall is one of the most useful features and
  principle-driven austerity isn't worth losing it. The package is
  explicitly *not* a pure CFG; we document what context we keep.
- **"Files are the source of truth."** Conflates with deployment.
  Left as a host concern per principle 2.
- **"Auto-cleanup is core."** It's English policy. Belongs as a
  pluggable post-processor (principle 3), not in the engine.
