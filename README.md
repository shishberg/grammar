# grammar

A Go package that parses and generates text from a rule-based grammar.
The format is inspired by [mezzacotta-generator][mezz] but is its own
thing — see [docs/principles.md](docs/principles.md) for the design
goals and [docs/design.md](docs/design.md) for the conceptual model and
public API. The formal source-format grammar is in
[docs/ebnf.md](docs/ebnf.md).

[mezz]: https://github.com/dangermouse-net/mezzacotta-generator

## Quick start

```go
import (
    "fmt"
    "math/rand"

    "github.com/shishberg/grammar"
)

const src = `rule greeting
  forms: default
  hello
  hi
  weight=2 hey

rule sentence
  forms: default
  {greeting}, world!
`

func main() {
    g, _ := grammar.Parse(src)
    rng := rand.New(rand.NewSource(1))
    out, _ := g.Generate("sentence", rng)
    fmt.Println(out) // e.g. "hey, world!"
}
```

## Inflection and back-reference

```
rule animal
  forms: default, plural={}s
  cat
  mouse | mice

rule story
  forms: default
  The {animal as A} chased the other {*A}.
  Two {animal:plural} walked into a bar.
```

`{animal as A}` saves the chosen expansion under `A`; `{*A}` recalls
it. The plural of `mouse` is `mice` because the entry overrides the
form-default `{}s` template.

## Post-processors

Language-specific tweaks are pluggable, not built-in. Two ship in
`postproc/english`:

- `english.AAn` — rewrites `a` to `an` before vowel-sounding words.
- `english.UnderscoreToSpace` — turns `fire_axe` into `fire axe` after
  inflection has run.

```go
out, _ := g.GenerateWith("story", rng, english.UnderscoreToSpace, english.AAn)
```

Hosts that don't speak English just leave them off.

## Command-line tool

`cmd/grammar` reads a directory of `.grammar` files, merges them, and
prints generations from a named rule:

```
go run ./cmd/grammar -dir ./examples/tavern -rule tavern -n 5 -seed 42 \
  -postproc underscore,aan
```

See `go run ./cmd/grammar -h` for flags.

## Status

Pre-1.0. The on-disk format and the public Go API may change.
