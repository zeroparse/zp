# Generating Parsers With `zp gen`

`zp gen` turns a grammar into Go source files: a parser package you commit to your repository, build with your program, and
call like any other package. By default the generated Go package depends only on the standard library: it carries a
copy of the ZeroParse runtime in its `internal/runtime` directory, and it does not load or compile the grammar at run time.

This guide shows how to generate a parser, what the generator supports compared with the engine (the `zp.Parser` of
[Running Parsers](running-parsers.md)), how to keep generated code up to date, and when it is the right choice. The API
of the generated Go package is in [The Generated Go API](generated-go-api.md), and the options that add typed values,
documents and streams have guides of their own, linked below. `zp gen -lang ts` generates
TypeScript modules with the same behavior instead; it has a guide of its own, [TypeScript parsers](typescript.md).

- [Why generate](#why-generate)
- [Quick start](#quick-start)
- [The `zp gen` command](#the-zp-gen-command)
- [What is supported](#what-is-supported)
- [Keeping generated code up to date](#keeping-generated-code-up-to-date)
- [Trade-offs](#trade-offs)
- [Recipes](#recipes)

## Why Generate

| | Engine (`zp.CompileSource`) | Generated parser |
|:--|:--|:--|
| Speed | Closure backend by default | Generated Go takes 0.44–0.61× closure time in CodePoints and 0.50–0.66× in Bytes; direct typed values take 0.69–0.98× generated `Parse` time ([benchmarks](../benchmarks.md), [analysis](../performance.md#where-zeroparse-stands)) |
| Dependencies | `zeroparse.dev/zp` | Standard library only (the runtime is copied into the package; `-runtime import` imports it from `zeroparse.dev/zp/runtime` instead) |
| Grammar at run time | Loaded and compiled (or loaded from `.zpc`) | Gone: it is compiled into Go code |
| Grammar changes | Edit the grammar, restart | Regenerate and rebuild |
| Recognition (no tree) | `RecognizeOnly` | `Recognize`, with [`-recognize`](generated-go-api.md#recognition--recognize) |
| Streaming, incremental, backends | Yes | Go Node `ParseStream` with `-stream` (`-types` adds `ParseStreamValue`); Go Node `Document` with `-document`; `-types` adds `Document.ParseValue`, using direct incremental matching for eligible grammars and Node conversion otherwise. No backend choice (see [what is supported](#what-is-supported)) |

Generate a parser when the grammar is fixed, the parser is on a hot path or in a small binary, or you want no
dependency on ZeroParse at run time. Use the engine when the grammar is data. For streams of Nodes, generate a Go
`ParseStream` with `-stream`; for incremental Node parsing, generate a Go `Document` with `-document`. The engine offers
the broader stream and Document APIs.
A good deal of the time you can use both: the engine in tools and tests, and the generated code in production
(see [Recipes](#recipes)).

The generated parser returns the **same tree and the same errors** as the engine, so switching between them does not
change what your code sees. The test suite checks this for every grammar in `examples/`
([design record 008](../design/records/008-code-generation.md)).

## Quick Start

A complete, runnable setup. The grammar, `pairs.zp`, is the one used in the [Running Parsers](running-parsers.md):

```zp
// pairs.zp
type Pair = { Key node[string]; Value node[string] }

@start(default)
rule main = ws ps:pair+ $$ => $ps
rule pair Pair = k:key "=" v:value ";"? ws => Pair{Key: $k, Value: $v}
@start
rule key = [a-zあ-ん]+ #atomic
rule value = [0-9]+ #atomic
rule ws = [ \t\n]*
```

The layout is a module with the grammar next to the code that generates from it, and the generated parser in its own
package (see [why its own package](#one-grammar-one-package)):

```text
example.com/pairs
├── go.mod
├── pairs.zp
├── generate.go
├── main.go
└── pairsparser/        # created by go generate
    ├── parser.go
    ├── api.go
    └── internal/runtime/   # and, with -types, types/types.go
```

Record `zp` as a tool dependency of the module, so that `go generate` runs the pinned version and nobody needs a
separate install:

```bash
go get -tool zeroparse.dev/zp/cmd/zp
```

This adds a `tool zeroparse.dev/zp/cmd/zp` line to `go.mod`. Then declare the generation step with a
`go:generate` directive; `zp gen` creates the output directory:

```go
// generate.go
package main

//go:generate go tool zp gen -g pairs.zp -module example.com/pairs/pairsparser -o pairsparser
```

```bash
go generate ./...
```

`-module` is the import path of the generated package, which the copy of the runtime in `pairsparser/internal/runtime`
is imported by; `-o` is the output directory, which `zp gen` creates. The result is `pairsparser/parser.go`, with the
code generated from the grammar, `pairsparser/api.go`, which re-exports the runtime names the users of the parser need
(`Node`, `SyntaxError` and so on), the runtime copy and, with `-types`, `pairsparser/types/types.go`, the package of the Go types of the grammar (the `-module` path is then required). `parser.go` starts with:

```go
// Code generated by zp v0.1.0. DO NOT EDIT.

// Package pairsparser is a parser generated from a ZeroParse grammar.
package pairsparser
```

The first line of every generated file names the version of `zp` that wrote it (`(devel)` for a `zp` run from a
checkout with `go run`; `go build ./cmd/zp` in a checkout stamps a pseudo-version of its commit instead), so that a parser can be checked against security advisories, which name affected versions the
same way. One version always writes the same header, so regeneration stays reproducible.

Now use it:

```go
// main.go
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"example.com/pairs/pairsparser"
)

func main() {
	node, err := pairsparser.Parse("abc=12; あい=3")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(node)
	for _, pair := range node.Elems() {
		key := pair.Field("Key").(*pairsparser.Node)
		fmt.Printf("%s %q at %d-%d\n", pair.Type(), key.Text(), key.Start(), key.End())
	}

	// Positions in UTF-8 bytes.
	node, _ = pairsparser.Parse("あい=3", pairsparser.WithUnit(pairsparser.Bytes))
	fmt.Println(node.Elem(0).End())

	// Any rule marked @start can be the start rule.
	node, err = pairsparser.ParseRule("key", "abc")
	fmt.Println(node, err)
	_, err = pairsparser.ParseRule("nope", "abc")
	fmt.Println(err)

	// Errors have the same fields as the engine's, but are types of the generated package.
	_, err = pairsparser.Parse("abc=x")
	var se *pairsparser.SyntaxError
	fmt.Println(errors.As(err, &se), se.Line, se.Col, se.Expected, err)

	out, _ := json.Marshal(node)
	fmt.Println(string(out))
}
```

```text
[Pair{Key: "abc", Value: "12"} Pair{Key: "あい", Value: "3"}]
Pair "abc" at 0-3
Pair "あい" at 8-10
8
"abc" <nil>
rule nope is not defined
true 1 5 [[0-9]] 1:5: syntax error: expected [0-9]
{"start":0,"end":3,"value":"abc"}
```

The output is exactly what the engine prints for the same grammar and input.

## The `zp gen` Command

```bash
zp gen -g <grammar> -module <import path> -o <directory> [-lang go] [-pkg <package>] [-runtime embed|import|only] [-s <rule>] [-types] [-document] [-stream] [-recognize] [-typed-only] [-recognize-only] [-expectations] [-observability] [-limits] [-source-info] [-default-value] [-nodoc] [-max-depth <n>] [-optimize speed|memory|size] [-opt <name[=count],...>] [-no-opt <name,...>] [-print-after <pass,...>]
zp gen -lang ts -g <grammar> -o <directory> [-runtime embed|import|only] [-runtime-module <specifier>] [-s <rule>] [-types] [-recognize] [-expectations] [-observability] [-limits] [-source-info] [-default-value] [-max-depth <n>] [-optimize speed|memory|size] [-opt <name[=count],...>] [-no-opt <name,...>] [-print-after <pass,...>]
zp gen -runtime only -o <directory> [-lang go|ts]
```

| Flag | Default | Meaning |
|:--|:--|:--|
| `-lang` | `go` | The language: `go`, or `ts` for TypeScript (see [TypeScript parsers](typescript.md)) |
| `-g` | (required, except with `-runtime only`) | The grammar: `.zp` or `.json` |
| `-module` | (required for Go with `-runtime embed`) | The import path of the generated package. The copy of the runtime is imported by `<module>/internal/runtime`. With `-runtime import`, `-pkg` can stand in for it, since the package then needs no import path of its own. |
| `-pkg` | last element of `-module`, without a major version suffix (`example.com/m/v2` gives `m`) | The package name of the generated code. It must be a valid Go identifier; a module path whose last element is not one (`my-parser`) needs `-pkg`. |
| `-runtime` | `embed` | How the generated code reaches the runtime: `embed` writes a copy (Go: `internal/runtime`; TypeScript: `runtime.ts`), `import` imports it (Go: `zeroparse.dev/zp/runtime`; TypeScript: the module of `-runtime-module`), and `only` writes just the copy, to refresh it without regenerating the parser. `only` takes only `-o`; a grammar or any other generation flag is an error |
| `-runtime-module` | (required for TypeScript with `-runtime import`) | The module specifier that the generated TypeScript imports the runtime from, for example `../shared/runtime.ts`. An error otherwise, and for Go |
| `-s` | the default start rule | The start rule of the generated `Parse` function |
| `-o` | (required) | The output directory (it is created). Generated files that the new run does not write again are removed, but `-runtime only` never removes `parser.go`, `api.go`, `types/types.go`, `parser.ts` or `types.ts`, and no file without the `// Code generated by zp <version>. DO NOT EDIT.` header is replaced or removed: `zp gen` stops with an error instead. |
| `-types` | off | Also generate the Go types for the grammar's types, in the package `<module>/types`, and `ParseValue` (see [typed values](generated-typed-values.md)) |
| `-document` | off | Also generate the incremental Node `Document` API (`createDocument` in TypeScript); with `-types`, it also exposes `Document.ParseValue` (`parseValue` in TypeScript) (see [incremental documents](generated-documents.md)) |
| `-stream` | off | Also generate the Go Node `ParseStream` (see [streams](generated-streams.md)); the start rule must have a `#stream` repetition; with `-types` it also generates `ParseStreamValue`, and it cannot be combined with `-lang ts` |
| `-typed-only` | off | Go only; needs `-types`. Generate only the typed entry points: `ParseValue` and `ParseRuleValue`, without `Parse` and `ParseRule` and without the code that builds untyped `*Node` values, which about halves the source of a large grammar. It can be combined with `-recognize` but not with `-document`, `-stream` or `-lang ts`. Entry points that convert the Node of their rule keep the unexported Node rule functions they run |
| `-recognize-only` | off | Go only. Generate only `Recognize`: no `Parse`, `ParseRule` or typed entry points. It cannot be combined with `-types`, `-typed-only`, `-document`, `-stream` or `-lang ts` |
| `-recognize` | off | Also generate `Recognize`, which checks input without building a tree (see [recognition](generated-go-api.md#recognition--recognize)) |
| `-limits` | off | Generate the limit-controlled form of the parser, which supports `WithContext` and finite `Compute` and `Memory` limits and exports `WithContext` from the package; in TypeScript it supports the `signal` option and the `compute` limit (see [TypeScript](typescript.md#cancellation-and-compute-limits)). Without it the Go parser holds the ordinary form of its functions and nothing that spends budget, which makes `parser.go` 15% to 50% smaller (the plain `parser.go` is 0.50 to 0.85 of the controlled one for the grammars of `testdata/grammars/`; larger grammars save more), and a parse that passes a context or a finite `Compute` or `Memory` returns an error; `Input` and `Depth` work in both forms (see [cancellation and compute limits](generated-go-api.md#cancellation-and-compute-limits)). A parser has one form: with the option it is the controlled one, and a parse that sets no limit runs the same code (a few inline tests of the control take the unmetered path), so generate two parsers if you need the faster plain form as well. A TypeScript parser generated without the option emits no code that spends budget and sets no control, and its options type and exports have no cancellation and no compute limit. With `-document` and without the option, the Document form has no charges |
| `-expectations` | off | Let a parse record what a syntax error expects: the expected list and the messages of `@error` (see [recording expectations](error-messages.md#recording-expectations)). The parser gets the table of expectation descriptions and the parse option `WithExpectations()`; a parse records only when it passes the option, and otherwise a syntax error has the farthest position, an empty `Expected` list, no `Messages` and the message `syntax error`. Without the flag the parser has no table and no option. A `Document` fixes the choice when it is created. It covers every entry point, including typed values, `Document` and streaming, and TypeScript (`{ expectations: true }`) |
| `-observability` | off | Emit opt-in tracing and profiling for Go or TypeScript parsers; combines with `-types`, `-recognize`, and Go `-document` and `-stream` (see [generated parser tracing and profiling](debugging.md#generated-go-node-and-typed-traces)) |
| `-max-depth` | `0` | Default nested recursive rule-call limit in the generated parser; zero selects 100,000, a positive value overrides it, and negatives fail generation |
| `-default-value` | off | Make the parser return the default node tree: value actions and declared rule types are dropped, predicates still run (see [Default Values](../../spec/language/actions.md#default-values)). Cannot be combined with `-types` |
| `-source-info` | off | Make the parser record the rule of each node and the names of captures, so that results can be printed with them (as `zp parse -source-info` does) |
| `-nodoc` | off | Leave out the package comment (`// Package p is a parser generated from a ZeroParse grammar.`), for a package that has its own documentation in another file |
| `-optimize` | `speed` | The optimization profile (see [Optimizations](#optimizations)). `speed`, `memory` or `size`; any other name is an error that lists the valid ones |
| `-opt` | none | Optimizations to give a value after the profile is applied and before `-no-opt`, separated by commas, as `name` or `name=count`: a name alone turns on an optimization that the profile leaves off (`-opt ts-literal-sharing`), a count sets an integer one (`-opt class-tables=3`), and `0` turns one off. An unknown name, a count that is not a number, or a value that the optimization does not take is an error |
| `-no-opt` | none | Optimizations to turn off after the profile is applied, separated by commas (names in [Optimizations](#optimizations)). A name that is unknown is an error; a known name that does not apply to the parser being generated does nothing |
| `-print-after` | none | Write the intermediate representation that the generator builds to standard error, as text, after the named steps and passes, separated by commas: `build`, a Core pass name, `flow`, a Flow pass name, `exec`, an Exec pass name, or `all` (see [Optimizations](#optimizations)); a pass that does not apply to the parser prints nothing |

Details:

- Both languages need `-o`, because the output is several files. TypeScript writes `parser.ts`, `runtime.ts` and, with `-types`, `types.ts`.
- The output is formatted with `gofmt`, passes `go vet`, and is **deterministic**: generating twice gives identical
  files. That is what makes the [up-to-date check](#keeping-generated-code-up-to-date) possible.
- Grammar errors are reported with positions, before any file is written. A rule with no expression is a syntax
  error:

  <!-- example: error -->
  ```zp
  // broken.zp
  @start
  rule main =
  ```

  ```bash
  zp gen -g broken.zp -module example.com/pairs -o /dev/null
  ```

  ```text
  zp: broken.zp:4:1: expected an expression, found end of file
  ```

  An action that reads a capture the rule does not have is a compile error:

  <!-- example: error -->
  ```zp
  // typeerr.zp
  @start
  rule main = x:[a-z]+ => $y
  ```

  ```bash
  zp gen -g typeerr.zp -module example.com/pairs -o /dev/null
  ```

  ```text
  zp: typeerr.zp:3:25: undefined capture $y
  ```

  ```bash
  zp gen -g pairs.zp -module example.com/pairs -s nope -o /dev/null
  ```

  ```text
  zp: pairs.zp: rule nope is not defined
  ```

  ```bash
  zp gen -g pairs.zp -module example.com/pairs -pkg foo-bar -o /dev/null
  ```

  ```text
  zp: pairs.zp: invalid package name "foo-bar"
  ```

- A `.zpc` cannot be used, because code generation needs the grammar and a compiled grammar holds only bytecode:
  `zp: pairs.zpc: a compiled grammar holds only the bytecode of one mode; gen needs the grammar source (.zp or .json)`.
- Without `-s`, the grammar's default start rule is used.
- With `-runtime import`, the module of the generated package requires `zeroparse.dev/zp/runtime`
  (`go get zeroparse.dev/zp/runtime@vX.Y.Z`, the version of `zp`). `parser.go` refers to the runtime protocol it was
  generated for (`const _ = rt.Protocol12`), so a runtime that does not implement that protocol fails the build with
  `undefined: rt.Protocol12` on that line. Regenerate the parser with the `zp` of the runtime version, or require a
  runtime version that implements the protocol.

### Choosing Entry Points

A Go parser has only the entry points you select. `Parse` and `ParseRule` build `*Node` trees and are there unless
`-typed-only` or `-recognize-only` leaves them out; `-types` adds the typed entry points; `-recognize` adds `Recognize`;
`-document` and `-stream` add the incremental and streaming ones. The typed-only and recognize-only forms leave out the
code that builds untyped trees, which is most of the source of a large grammar. Combinations that cannot work are
errors: `-typed-only` needs `-types` and has no `-document` or `-stream`, which build untyped trees, and `-recognize-only`
has nothing else. The sizes below are the kilobytes of the generated `parser.go` (with `-nodoc -runtime import`) of
six language grammars.

| Grammar | `-types` | `-types -recognize` | `-types -typed-only` | `-types -typed-only -recognize` | `-recognize-only` |
|:--|--:|--:|--:|--:|--:|
| cel | 213 | 293 | 115 | 195 | 89 |
| cue | 782 | 1,091 | 419 | 729 | 331 |
| duckdb | 3,749 | 5,370 | 1,747 | 3,369 | 1,719 |
| golang | 1,109 | 1,620 | 597 | 1,107 | 548 |
| python | 1,207 | 1,763 | 657 | 1,212 | 604 |
| typescript | 1,893 | 2,767 | 1,038 | 1,916 | 959 |

### Optimizations

The generator optimizes what it emits. The contract of the optimization settings (the order in which they apply, their values, and the guarantee that none changes what a parser returns) is in [Optimization Settings](../../spec/runtime/generated-parsers.md#optimization-settings); this section lists the optimizations. Each optimization is a flag with a name. A profile is a preset: a name for fixed
values of flags, with no logic of its own. The `speed` profile, the default, has every flag on except the opt-ins named
below (`ts-literal-sharing` and `class-tables`). `-opt` sets a flag, and `-no-opt` turns flags off after the profile and `-opt` are applied, so `-no-opt` wins.
A flag keeps what the parser returns, the same results, errors and events, and changes the code, the speed and the size.
A flag that changes the generated API or the representation of the results instead is marked "changes output" in
the table, is always an opt-in, and is never part of a profile. Two uses of the flags:

- To isolate a miscompilation: generate with `-no-opt` and one name and see whether the output changes.
- To measure an optimization with the same generator on both sides.

The kind says what the optimization changes: a `pass` rewrites the parser before it is lowered, a `lowering` is an
option of the lowering to the code, a `printer` is a choice of what the Go and TypeScript printers emit, and a `runtime`
optimization is a control of the runtime structures the parser emits. The names are the following, and an unknown name in `-opt` or `-no-opt` is an error that lists them. No flag in this
table changes output yet:

| Name | Kind | Default | What it does and what it costs |
| --- | --- | --- | --- |
| `terminals` | pass | on | Value-free choices of many literals and classes run as one program (lean typed Go). Data against code: the choice becomes a table in the parser instead of nested alternatives |
| `text-dispatch` | lowering | on | A predicate that compares the text of a capture with literals tests them at once when the parse runs without limit controls. One test of the literals against a chain of comparisons, and the text is computed once |
| `field-compare` | lowering | on | Typed Go compares fields in the actions and predicates of typed rules without boxing string fields. Fewer allocations, no cost |
| `guards` | pass | on | Skip an alternative when the next character cannot start it. One read of the next character per choice against the attempts of alternatives that cannot match, and a little more code per choice |
| `guarded-classes` | pass | on | A leading class of a guarded alternative consumes the character its guard read (ordinary Go parsers). Saves a second read and test of that character |
| `scans` | pass | on | A value-free repetition of a class or of any character runs as a scan. A loop against the general repetition |
| `class-runs` | pass | on | Consecutive classes of a value-free sequence run as one (lean typed Go). One run against one test and call per class |
| `class-tables` | lowering | off | Match a class of at least this many ranges through a membership table (lean typed Go); `-opt class-tables=5` is the measured choice, and 0 turns the tables off. Table data against code: about 3% smaller source, and about 1% slower parsing on the language grammars (3.7% on TypeScript). It is an opt-in |
| `inline-literals` | lowering | 4 | Compare literals of at most four code points in place first; the number is the longest literal, and 0 turns it off. Code against calls: faster, with more code |
| `method-sharing` | printer | on | Emit one helper for generated bodies that are exactly equal (Go and TypeScript). Code against calls: smaller source, with a call where a body was |
| `literal-sharing` | printer | on | Emit one table for equal literals (Go; TypeScript also needs `ts-literal-sharing`). Smaller source against an indirection to the table |
| `ts-literal-sharing` | printer | off | Share literal tables in TypeScript parsers. Smaller code against speed: a small cold parse measured about 1.5 times slower. It is an opt-in: turn it on with `-opt ts-literal-sharing` |
| `typed-object-slabs` | runtime | on | Allocate typed objects in slabs (typed Go). Fewer allocations against memory: an object that escapes keeps the unused objects of its chunk |
| `typed-list-slabs` | runtime | on | Allocate typed lists in slabs (typed Go). Fewer allocations against memory, as for objects |
| `typed-output-bootstrap` | runtime | on | Size the first slab of the typed output from the input (typed Go). A small parse keeps little memory, against a few allocations before the slabs start |
| `sparse-memo` | runtime | on | Keep the first-call table of large TypeScript parsers sparse. Memory in proportion to the positions visited against a page lookup per check |

A profile is chosen with `-optimize` (`Optimize` in `gen.Go` and `gen.TypeScript`). Each sets flags of the table above to other values
than `speed` has and nothing else, so `-opt` and `-no-opt` still apply after it:

| Profile | Sets | For | Measured against `speed` (ratios to `speed`, geometric mean over ten language grammars) |
| --- | --- | --- | --- |
| `speed` | nothing | The default: the fastest parse that does not cost generated code or memory out of proportion | |
| `memory` | `typed-object-slabs` and `typed-list-slabs` off | Fewer bytes and less live heap in typed Go parsers, at the price of many more allocations | bytes allocated 0.886 and live heap 0.848 of `speed`'s, parse time 0.996; allocations rise 1.03 to 1.39 times for the typed grammars and 52 to 97 times for the JSON, CSV and XML parses; no grammar is more than 3.8% slower |
| `size` | `inline-literals` off, `class-tables` at 5 | Smaller generated Go source and binaries | `parser.go` 0.912, stripped binary 0.986 and parse time 1.022 of `speed`'s; no grammar is more than 5.5% slower |

No profile changes what a parser returns. The measurements are in [optimization record 136](../optimizations/136-measured-effect-of-the-generation-flags.md).

A flag never overrides soundness: whether an optimization applies to a parser is decided by the generator, and `-opt`
never turns on a pass for a parser that it is unsound for. `-no-opt guards` also leaves out the guarded classes, which need guards.

`-print-after` shows the Core IR, the form the passes work on, for debugging the generator. A pass that does not apply to the
parser (the lean passes in a parser with limit controls, for example) prints nothing. Only the generation that
writes the output prints, and the text starts with `-- Core IR of the <table> after <pass> --`, where `<table>` is
`typed` for the typed rules, and `rules` or `recRules` for the untyped and recognizer tables. `-print-after flow` shows
the next form: each function that the parser is printed from, in Flow IR, as lowered from the Core IR, under
`-- Flow IR of <function> in the <table> table after flow --`, and `-print-after exec` the form after that, Exec IR,
under `-- Exec IR of <function> in the <table> table after exec --`; the passes over Flow IR and Exec IR print under
their own names.

### From Go

The same generator is available from Go, in package `zeroparse.dev/zp/gen`, for build tools that produce parsers programmatically:

```go
g, err := zp.ParseGrammar(src)
if err != nil {
	log.Fatal(err)
}
files, err := gen.Go{Module: "example.com/pairs/pairsparser"}.Generate(g, "main") // grammar, start rule
if err != nil {
	log.Fatal(err)
}
for _, f := range files { // parser.go, api.go, types/types.go with Types, and internal/runtime/*.go
	path := filepath.Join("pairsparser", filepath.FromSlash(f.Path))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(path, f.Content, 0o644); err != nil {
		log.Fatal(err)
	}
}
```

The fields of `gen.Go` are the flags of `zp gen`: `Module` is `-module`; `Types: true` is `-types`; `Document` is `-document`; `Stream` is `-stream`; `Recognize` is `-recognize`; `TypedOnly` is `-typed-only`; `RecognizeOnly` is `-recognize-only`; `LimitControls` is `-limits`; `Observability` is `-observability`; `SourceInfo` is `-source-info`; `DefaultValue` is `-default-value`; `OmitPackageDoc` is `-nodoc`; `Runtime: gen.RuntimeImport` is `-runtime import`; `PackageName: "p"` is `-pkg p`; and `MaxDepth: 600_000` is `-max-depth 600000`; `Optimize` is `-optimize`; `Opt: map[string]int{"class-tables": 3}` is `-opt class-tables=3`; `NoOpt: []string{"guards"}` is `-no-opt guards`; and `PrintAfter` with `PrintTo` is `-print-after` (it needs a writer, such as `os.Stderr`). `gen.TypeScript` has the same five fields. Omitted, zero and explicit 100,000 defaults generate identical source. Observability supports standalone Go Node and typed batch parsers, including `ParseValue` and `ParseRuleValue`, Go Documents and streams, and TypeScript batch parsers with or without `-types`. Generated Go Node, typed and Document packages expose `WithProfile`; TypeScript batch packages expose `Profile` through per-call options. Typed direct execution and Node-conversion fallback report their actual rule calls; conversion itself has no separate event. Invalid defaults return an error before generated output is written.

`zp gen` is a thin wrapper around `gen.Go` (and `gen.TypeScript` with `-lang ts`, whose `Generate` returns the files too); both implement `gen.Generator`. Because a grammar built with the `grammar` package (see the
[Running Parsers](running-parsers.md#from-go-values-programmatic-grammars)) is just an AST, it can be generated from, too.

When `-observability` is enabled, the generated Go or TypeScript package
includes opt-in rule tracing and profiling. See [generated Go Node and typed
traces](debugging.md#generated-go-node-and-typed-traces), [generated
TypeScript traces](debugging.md#generated-typescript-traces), and [generated
TypeScript profiling](debugging.md#profiling-generated-typescript-batch-parsers)
for the contracts and examples.

The ordinary generated source is unchanged when this option is off. Enabling
it adds generated code and some parse overhead even when no callback is passed;
delivering events costs more. Focused Node measurements showed no-callback
parse ratios around 1.01–1.06× and recognition around 1.02–1.20×. Typed
no-callback ratios ranged from 0.995–1.075×; scalar-store callbacks ranged
from 1.169–3.164× depending on workload and execution path. These are synthetic
measurements, not workload-wide estimates. Focused `WithProfile` runs were
3.8–26.9× ordinary time and added about 45–395 KB/op and 25–50 allocations/op
across the measured cases; see
[design 043](../design/records/043-generated-parser-observability.md#performance-and-results)
for scope and generation costs. Focused TypeScript profiles measured 1.58–4.02×
ordinary time, depending on API and fixture; see the TypeScript profile
measurements in design 043 for conditions and limits.

## What Is Supported

The generator supports the whole grammar language: Rule Expressions, Operators Rules, left recursion, Predicates,
variables, actions, types, built-in functions, `@error` and `#recover`. What differs from the engine is the runtime
machinery, which the generated code only has when you ask for it:

- `-stream`, `-document` and `-recognize` add `ParseStream`, `Document` and `Recognize` (and `-types` their typed
  forms). Without `-stream`, `#stream` is an ordinary repetition. TypeScript has neither streams nor documents.
- `ParseRule` replaces `WithStart` for another start rule, and `WithUnit` and `Limits.Depth` work as in the engine,
  except that `Depth: Unlimited` is an error; `-max-depth` sets the generated default.
- Cancellation and the compute limit are `WithContext` and `Limits.Compute` in the engine and in generated Go with `-limits`,
  and the `signal` option and the `compute` limit in TypeScript with `-limits`; the input limit is `Limits.Input` and
  `limits.input` in every form. See
  [Cancellation and Compute Limits](cancellation-and-limits.md) and the
  [specification](../../spec/runtime/generated-parsers.md#compatibility-and-support).
- There is no choice of backend, because the generated code is a backend of its own, and no `Parser.Grammar()`, saving
  or loading, because the grammar is gone at run time.
- `#missing` works only in the engine's Closure backend for whole-input parsing and recognition, and the generator
  rejects grammars that use it.

The [specification](../../spec/runtime/generated-parsers.md#compatibility-and-support) has the complete table.

Notes:

- **Depth.** Generated code uses the Go stack for rule calls and fails with
  `nesting too deep: more than 100000 nested recursive rule calls` at the default limit. `Limits.Depth` changes the limit for one
  invocation, and `-max-depth` changes the generated default. With the nested-parentheses grammar
  `rule nested = "(" nested ")" / "x"`, 50,000 levels parse and 200,000 levels fail with that error. If you must accept
  deeper nesting beyond the host stack, the engine's `BytecodeIterative` backend with `Limits.Depth` is the tool
  ([Choosing a Backend](backends.md#deep-nesting-and-the-depth-limit)).
- **Memory.** Generated parsers memoize in the same way as the engine, so memory still grows with the input. For input
  larger than memory, generate `ParseStream` with `-stream` or use the engine's `ParseStream`.
- Using a stream grammar with the generator is fine: `Parse` treats the `#stream` pragma as an ordinary repetition, so
  the same grammar file serves both streaming and whole-input parsing.

## Keeping Generated Code Up to Date

Generated code is source: commit it, review its diffs, and regenerate it whenever the grammar changes **or you upgrade
ZeroParse** (the embedded runtime comes from the version of `zp` that generated it). Three habits keep it honest.

**1. Generate with `go generate`.** Put the directive next to the grammar, as in [Quick start](#quick-start), and run `go generate ./...`. Pin the generator with
`go get -tool` as shown above. (`go install zeroparse.dev/zp/cmd/zp@latest` and
`//go:generate zp gen ...` works, too, but every developer then has to install a matching version.) With several grammars, add one directive per grammar.

**2. Test that it is up to date.** A test regenerates in memory and compares with the committed file; it fails with a
reminder when somebody changed the grammar and forgot to regenerate:

```go
//go:embed pairs.zp
var grammarSrc string

// The files of pairsparser are up to date with pairs.zp.
func TestGeneratedIsUpToDate(t *testing.T) {
	g, err := zp.ParseGrammar(grammarSrc)
	if err != nil {
		t.Fatal(err)
	}
	want, err := gen.Go{Module: "example.com/pairs/pairsparser"}.Generate(g, "main")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range want {
		got, err := os.ReadFile(filepath.Join("pairsparser", filepath.FromSlash(f.Path)))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(f.Content) {
			t.Errorf("pairsparser/%s is out of date; run go generate ./...", f.Path)
		}
	}
}
```

If you change `pairs.zp` (here: `rule value =  [0-9a-f]+ #atomic `) and skip regeneration, the test fails. Among the output:

```text
--- FAIL: TestGeneratedIsUpToDate (0.00s)
    generated_test.go:49: pairsparser/parser.go is out of date; run go generate ./...
FAIL
```

Run `go generate ./...` in your project and check that the generated files have no diff (for example, with `git diff --exit-code`). This makes stale output fail in your own CI.

**3. Test that it behaves like the engine.** The engine is the reference implementation, and a one-time parity test
protects you from the two diverging (for example after an upgrade):

```go
func TestGeneratedMatchesEngine(t *testing.T) {
	p, err := zp.CompileSource(grammarSrc, "main")
	if err != nil {
		t.Fatal(err)
	}
	for _, in := range []string{"abc=12", "abc=12; あい=3", "", "abc=", "abc=1 x"} {
		want, werr := p.Parse(in)
		got, gerr := pairsparser.Parse(in)
		wj, _ := json.Marshal(want)
		gj, _ := json.Marshal(got)
		if string(wj) != string(gj) || fmt.Sprint(werr) != fmt.Sprint(gerr) {
			t.Errorf("%q:\n engine    %s %v\n generated %s %v", in, wj, werr, gj, gerr)
		}
	}
}
```

Comparing JSON and error text covers the whole tree (with positions) and all error details. Include inputs that fail,
and inputs that exercise each feature of your grammar.

## Trade-Offs

- **Speed.** In the current [benchmark results](../benchmarks.md), generated Go takes 0.44–0.61× closure time in
  CodePoints and 0.50–0.66× in Bytes across eight workloads. Direct `ParseValue` values take 0.69–0.98× generated
  `Parse` time; conversion workloads that include generic nodes can take longer. Standard-library comparisons produce
  different outputs and remain faster where they apply. These measurements are from one machine and commit; run
  `go run ./bench/report` to measure yours.
- **Binary and repository size.** Each generated parser adds source that you commit and that compiles into your
  binary: `parser.go` and `api.go`, with `-types` `types/types.go`, and with `-runtime embed` a copy of the runtime in `internal/runtime` (the same for
  every grammar; `-runtime import` shares one copy through `zeroparse.dev/zp/runtime` instead). It does not pull in
  ZeroParse's compiler, analyzer or VMs.
- **Start-up.** There is none: no grammar to parse or compile. If start-up time is the only concern and a source file
  is acceptable, loading a [`.zpc`](compiled-grammars.md) is the cheaper alternative.
- **Flexibility.** The grammar is frozen into the binary. Changing it means regenerating, rebuilding and redeploying;
  if users supply grammars at run time, use the engine.
- **Features.** No backend choice; streaming requires `-stream` and incremental Node parsing `-document` (see
  [what is supported](#what-is-supported)).
- **Review noise.** Large diffs on regeneration (particularly after upgrading ZeroParse) are expected. Mark the files as
  generated for your review tool: the first line, `// Code generated by zp <version>. DO NOT EDIT.`, follows the Go
  convention that tools recognize.
- **Typed values.** `Parse` returns generic `*Node` trees like the engine, which is what makes the two interchangeable.
  For Go types matching the grammar's types, generate with [`-types`](generated-typed-values.md) and call `ParseValue`;
  for types of your own, write a small conversion from `*Node` or from the typed values, as
  [alpha-parsers/json](https://github.com/zeroparse/alpha-parsers/blob/main/json/json.go) does (`ToAny`).

## Recipes

### One Grammar, One Package

Each generated package defines `Node`, `SyntaxError`, `Parse` and the rest at package level, so two generated parsers in
the same package collide:

```text
both/b.go:17:6: Node redeclared in this block
	both/a.go:17:6: other declaration of Node
```

Give every grammar its own package (and directory), as the ready-made parsers of `alpha-parsers` do. If you also
want several entry points of one grammar, use `ParseRule` rather than generating it twice.

### Engine in Development, Generated in Production

Keep one `.zp` file. Use `zp parse` and the engine (`CompileSource`) in tools, experiments and tests, where start-up
and features matter more than speed; generate the production parser from the same file; and let the
[parity test](#keeping-generated-code-up-to-date) prove they agree.

### Choosing the Start Rule

Mark every rule where parsing may begin with `@start`, and the usual entry rule with `@start(default)`, so that
`Parse` is the common case; `-s` (for example `zp gen -g lang.zp -module example.com/lang -s program`) selects
another start rule for `Parse`. Use `ParseRule("expr", input)` for the other start rules, for example for an
expression evaluator embedded in a larger tool.

### Wrapping the Result in Your Own Types

If the grammar declares the types you want, [`-types`](generated-typed-values.md) generates them. Otherwise: the generated
`Node` is the same loose shape for every grammar. A thin layer keeps that out of the rest of your program:

```go
type Pair struct{ Key, Value string }

func Pairs(input string) ([]Pair, error) {
	node, err := pairsparser.Parse(input)
	if err != nil {
		return nil, err
	}
	var out []Pair
	for _, n := range node.Elems() {
		out = append(out, Pair{
			Key:   n.Field("Key").(*pairsparser.Node).Text(),
			Value: n.Field("Value").(*pairsparser.Node).Text(),
		})
	}
	return out, nil
}
```

Design the grammar's types and actions ([Declaring Types](types.md), [Actions](actions.md)) so that this layer is
thin, and unit-test it with the engine and the generated parser alike.

## Related Topics

- [The Generated Go API](generated-go-api.md): `Parse`, recognition, call options, cancellation and compute limits.
- [Typed Values in Generated Go](generated-typed-values.md), [Incremental Documents in Generated Parsers](generated-documents.md)
  and [Streams in Generated Go](generated-streams.md): the packages that `-types`, `-document` and `-stream` add.
- [Generated TypeScript Parsers](typescript.md): the same generator for TypeScript.
- [Compiled Grammars](compiled-grammars.md): the other way to skip compiling at run time.
- [Using the Ready-Made Parsers](ready-made-parsers.md): parsers made with this generator.
