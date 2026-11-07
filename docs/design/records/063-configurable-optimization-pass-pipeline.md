# 063. Configurable Optimization Pass Pipeline

- **Status**: Accepted
- **Author**: @ornew
- **Date**: 2026-10-25

## Summary

The passes over Core IR form a pipeline that a small manager runs, and every generation-time optimization is a named
flag in one registry:

- each pass declares its name, the analyses it requires and invalidates, its mode applicability and its obligation;
- a flag per optimization turns a pass or one of its variants off;
- an optional verifier runs between passes;
- a profile (`speed` today, later `memory` and `size`) is a named flag set;
- the module can be printed after any pass.

The default pipeline produces the generated output of the former hand-written sequences byte for byte. Users select a
profile with `zp gen -optimize`, remove optimizations with `-no-opt`, and print the Core IR with `-print-after`.

## Motivation

Before the pipeline the passes were called by two hand-written sequences and the generator decided which ran through
scattered booleans.

- The sequence was written twice, in the typed Go route and in the route of the Node and recognizer tables. The order
  matters and was nowhere declared: `Guards` reads what `Analyze` computes, and a missing `Analyze` leaves choices
  unguarded without an error (the DuckDB grammar made 84,000 calls to rules of later alternatives that a guard skips).
- Applicability was derived in several places: lean (no budget controls and no Documents), observability and the
  switches of the options. A pass such as `ClassRuns` is sound only in lean builds, because budget controls and
  Documents charge for reading; that obligation was encoded by an `if`, not stated.
- Only some optimizations had a switch, so no test checked that generation without `Guards`, `Scans` or `ClassRuns`
  equals the engine.
- `Verify` ran once, after the last pass; a pass that broke an invariant was reported at the end, with no pass named.
- There was no way to print the module after a pass from a command.
- Generation profiles did not exist. The size and speed of generated parsers are decided by one fixed set of
  optimizations ([record 062](062-backend-independent-optimization-ir.md) lists the measured mechanisms); a user who
  wants a smaller parser has no lever except removing features.

Record 062 requires every optimization to be a pass with a stated precondition, a private same-source switch and a
negative-control test. This record gives that rule a mechanism.

## Goals

- One declaration per pass: name, requirements, invalidations, applicability, obligation and the flag that controls it.
  Both routes that run Core passes run the same pipeline.
- The default pipeline produces the same Core IR, the same Flow IR and the same generated source as the former
  sequences for every grammar and generator mode.
- Every optimization has a flag, and a test that a build without it equals the engine.
- A pass that breaks a Core IR invariant is named, when verification between passes is on.
- The Core IR can be printed after any pass.
- A profile is a named flag set; `speed` is the default.

## Non-Goals

- Passes over Flow IR and Exec IR. Neither has an optimization pass that a user can switch: Flow lowering makes its
  choices through the flags `class-tables` and `inline-literals`, and the Exec IR passes (`Specialize`, `Simplify`,
  `DeadStores`) are part of lowering the emitted code and are not optional; for example, Go rejects a variable that is
  declared and not used, so dead stores must go. They stay lowering flags and required analyses. The manager is generic over the unit it runs
  on and can host Flow or Exec passes when one becomes optional.
- The textual pass that rewrites generated Go (`QualifyRuntime`). It stays a text pass: record 062 moves the
  other rewrites into the printers and leaves the cost of this one to an independent fix.
- The closure backend and bytecode, which do not use Core IR. TypeScript uses the Core passes through the route of the
  Node and recognizer tables and has the flags of its own runtime options.
- Changes to what a parser does. Flags and profiles change generated code, never results, errors or events.
- A textual pipeline language such as LLVM's `-passes=`.

## Design

### Pass Declarations

A pass is a value of `pipeline.Pass[M, X]` (`internal/ir/pipeline`), generic over the unit `M` and the mode `X`; the
Core passes are declared in `internal/ir/core/pipeline.go`. The names are those of the pipeline code, not of any public
API.

| Field | Meaning |
|:--|:--|
| `Name` | Stable identifier, also the flag name and the print-after name |
| `Requires` | Analyses that must be valid when the pass starts |
| `Invalidates` | Analyses that the pass makes stale |
| `After` | Passes that must precede it when both are enabled (an ordering constraint with no data dependence) |
| `Applies` | A predicate over the generation mode (`core.Mode`: lean, ordinary, observe) |
| `Obligation` | One sentence stating what the pass preserves |
| `Run` | The function over the unit |

Analyses are the data that passes read and that other passes make stale. The Core pipeline has one: `attributes`
(`core.Analyze`: `Nullable`, `First`, `Eff`). The attributes that the builder sets (`Want`, `Shape`) are inputs, not
analyses. No pass changes the calls between rules, so the call graph is checked by `Verify` and is not an analysis of
the manager.

The five Core passes, declared:

| Pass | Requires | Invalidates | After | Applies when | Obligation |
|:--|:--|:--|:--|:--|:--|
| `terminals` | | `attributes` | | Typed table, lean, no observability | Same input consumed and expectations recorded as the expression it replaces |
| `term-rewrites` | | | | Always | Same results and errors, in the same order |
| `guards` | `attributes` | | `terminals` | Always; guarded classes only when ordinary | A guard equals the alternative's first terminal |
| `scans` | | `attributes` | | Always | Same input consumed and expectations recorded |
| `class-runs` | | `attributes` | `guards` | Typed table, lean | Same input consumed and expectations recorded; the first class of a guarded alternative is left alone |

`class-runs` reads the `Want` attribute and the guard marks, not the attributes that `Analyze` computes, so it requires
no analysis; it is declared `After guards` because it must leave the first class of a guarded alternative alone.
`Analyze` is not a pass but the way the manager makes `attributes` valid. It is deterministic over an unchanged tree, so
how often it runs never changes the output.

### The Manager

`Pipeline.Run(m, x, options)` runs the enabled passes in order. Every analysis starts stale, because the builder does
not compute them. Before each pass the manager makes the pass's required analyses valid, and after a pass it marks the
invalidated ones stale. `Validate` rejects a pipeline in which a pass requires an analysis that the pipeline cannot
compute, or names an `After` pass that does not exist or follows it. The run fails with the name of the pass that
failed, or `verification` for the final check.

On the typed table with every optimization enabled, `attributes` is stale at the start and is computed before `guards`;
it is stale again after `scans` and `class-runs` and is recomputed once before the final verification. The pipeline
of the recognizer table and of the Node table is the same list in a mode that is not lean, so `terminals` and
`class-runs` do not apply there. Where a route needs to follow what a pass does, it hooks the pass: the route of the Node
and recognizer tables keeps the origin of each repetition that `scans` replaces (`core.Env.PrepareScans`).

The mode is a value the generator passes in (`core.Mode`), so `core` keeps no knowledge of budget controls or
Documents.

### Flags

The registry `internal/gen/opt` lists every optimization with a name, a kind (`Pass`, `Lowering`, `Runtime`), a
description and its value in the `speed` profile. The Core pipeline names the passes, and a test checks that the
registry has the name of every pass.

| Flag | Kind | Controls |
|:--|:--|:--|
| `terminals` | pass | `terminals` |
| `term-rewrites` | pass | `term-rewrites` |
| `text-dispatch` | pass | The `TextDispatch` rewrite of `term-rewrites` |
| `field-compare` | pass | The `FieldCompare` rewrite of `term-rewrites` |
| `guards` | pass | `guards` |
| `guarded-classes` | pass | The guarded-class marks that `guards` sets, and their lowering |
| `scans` | pass | `scans` |
| `class-runs` | pass | `class-runs` |
| `class-tables` | lowering | `lower.Options.ClassTables` |
| `inline-literals` | lowering | `lower.Options.InlineLiterals` (a count, 4 in `speed`; 0 turns it off) |
| `typed-object-slabs` | runtime | Typed objects are allocated in slabs |
| `typed-list-slabs` | runtime | Typed lists are allocated in slabs |
| `typed-output-bootstrap` | runtime | The first slab of the typed output is sized from the input |
| `sparse-memo` | runtime | The first-call table of large TypeScript parsers is sparse |

A flag is a value: 0 or 1, or a count. A profile may set a flag that `speed` leaves out, an opt-in, and `-no-opt` only
removes. A flag can only turn an optimization off: turning it on never overrides `Applies`, so `class-runs` stays off in
a build with budget controls whatever the flag says, because the pass is unsound there. The soundness gate is the
pass's `Applies`, never a flag. A known name that does not apply to the parser being generated is accepted and does
nothing; an unknown name is an error that lists the valid ones. The mode facts of `lower.Options` (`Spend`, `Recovers`,
`Observe`) are not optimizations and have no flag, and neither does `DeadStores`, which the emitted code needs.

`guards` off implies `guarded-classes` off and makes `class-runs` convert every class run, which `After` allows.

### Verification Between Passes

With verification on (`GenOptions.VerifyEachPass`), the manager makes `attributes` valid and calls `core.Verify` after
every pass and names the pass that produced the first violation. Off (the default), it verifies once at the end.
Verification between passes can run more analyses than the default pipeline; the output does not change. The test
helpers of the generator set it, so every fixture of the suite runs with the check between passes.

### Profiles

A profile is a named set of flag values. `speed` is the default and sets every flag on, with `inline-literals` at 4.
`-optimize` takes a profile name, and any other name is an error until the profile is registered.

`memory` and `size` are not registered. Their cells are hypotheses until measured: a table is data and a straight-line
expression is code, and the first may be smaller or larger depending on the grammar; guards add code and remove calls.

| Flag | `speed` | `memory` | `size` |
|:--|:--|:--|:--|
| `terminals` | on | on | open |
| `term-rewrites`, `text-dispatch`, `field-compare` | on | on | on |
| `guards`, `guarded-classes` | on | on | open |
| `scans` | on | on | on |
| `class-runs` | on | on | open |
| `class-tables` | on | open | off |
| `inline-literals` | 4 | open | 0 |
| `typed-object-slabs`, `typed-list-slabs`, `typed-output-bootstrap`, `sparse-memo` | on | open | open |

Each cell marked open is filled by measuring generated source size, binary size and resident memory on the checked-in
parsers, and the measured result goes in the profile's table and in `docs/optimizations/`. Adding a profile is a row
in the registry, a consumer of the profile name, and a measured cell for every open one.

The word "profile" already names the rule profiler (`zp profile`, `WithProfile`), so the flag that selects a profile is
`-optimize`.

### Print After Pass

The manager takes an optional `Dump func(pass string, m M)`, called with `build` before the first pass and with the name
of each pass that ran, after it. The generator prints with `core.Print(m, true)` to `PrintTo` after the passes selected
by `PrintAfter`: a comma-separated list of `build`, pass names, or `all`, as in LLVM's `-print-after`. Only the
generation that writes the output prints. Each section starts with `-- Core IR of the <table> after <pass> --`, where
`<table>` is `typed` for the typed rules, and `rules` or `recRules` for the Node and recognizer tables. The tests use
the same hook.

### User-Facing Surface

Three surfaces were considered.

**Option A: Internal only.** Flags exist for tests and diagnostics through private fields. `zp gen` and the Go options
do not change. Profiles are not offered.

**Option B: Profiles.** `zp gen -optimize speed|memory|size` and a Go option select a profile. Per-pass flags stay
internal.

**Option C: Profiles and per-optimization flags.** Option B plus `zp gen -no-opt name[,name]` for reproducing a
miscompilation or measuring one optimization, and `-print-after name|all` for the IR dump.

Decision: **C, with the profile flag named `-optimize`.** The Go API mirrors the flags in the fields of `gen.Go` and
`gen.TypeScript`, as `Types` mirrors `-types`: `Optimize string`, `NoOpt []string`, `PrintAfter string` and
`PrintTo io.Writer`. A profile is applied first and `-no-opt` removes optimizations from it, so `-optimize size -no-opt
guards` is `size` without `guards`. `PrintAfter` without `PrintTo` is an error. The name `-optimize` avoids the rule
profiler's `zp profile` and `WithProfile`. The flags and the names are described in the
[code-generation guide](../../guide/code-generation.md#optimizations); the
[specification of generated parsers](../../../spec/runtime/generated-parsers.md) does not change, because it constrains
the parser's behavior, not the generated code. Optimization names follow the pass set, which record 062 expects to
change.

Any option leaves the generated API, results, errors and events unchanged.

## Alternatives Considered

**Keep the call sites as they were, and add the missing switches.** Cheapest, and fixes the missing negative controls.
It leaves the order undeclared and the duplication in place, so the next pass is added in two places and the
wrong-order failure remains silent. Rejected.

**A fixed list without analyses.** A pipeline as a slice of functions, ordered by hand, with each pass calling
`Analyze` itself. Smaller than a manager. It spends analysis time where a pass might not need it and cannot express a
requirement, so it cannot report the silent mis-ordering that motivated this record. Deferred, not excluded: if
declared requirements prove not to be worth their code, this is the fallback.

**Full LLVM-style analysis management.** Cached analyses with preserved-set computation, per-function and per-module
scopes, and a textual pipeline language. The one analysis that exists (`attributes`) is cheap to recompute and
deterministic, so a cache with preservation sets buys little; per-rule scopes would matter only if passes became
rule-local. Rejected for now.

**Profiles chosen by the generator from the grammar.** The generator could pick `size` or `speed` by grammar size.
It makes the output depend on a heuristic that users cannot predict or override. Rejected.

**Flags only on the command line, with no Go option.** The Go API (`gen.Go`) is what the build tooling of other
projects calls, and the options mirror each other (`-types` and `Types`). The new ones mirror each other too.

**A functional-options form (`WithOptimize`, `WithoutOptimizations`).** `gen.Go` and `gen.TypeScript` are structs of
plain fields, and their options are fields. Rejected for consistency.

## Testing

- Pass declarations: `Pipeline.Validate` rejects a pipeline whose requirements cannot be computed or whose `After`
  constraints name a missing or later pass, and the Core pipeline is valid.
- Registry: `TestOptimizationsHaveTests` fails for an optimization without a description or a test of its disabled
  form, and for a test that names a flag that does not exist. The guide lists every registered optimization.
- Disabled optimizations: for each flag, lean typed parsers generated without it run the generated-parser differential
  (`GeneratedTypesCorpus`), which compares results and errors with the engine in both position units. Each
  flag is also checked to change the generated code or the Core IR on the cases, so a flag that does nothing is caught.
  `term-rewrites` is the case of the second kind: no printer reads the rewritten terms, so only the Core IR shows it.
- Combinations: all optional flags off, and a seeded sample of pairs, with `Verify` after every pass. The full 2^n
  product is not run.
- Mode applicability: `class-runs`, `terminals` and guarded classes do not run in builds with work controls, Documents
  or (for `terminals`) observability, whatever the flags say.
- Manager: a pass that requires a stale analysis gets it recomputed; a pass that violates `After` fails validation;
  verification names the pass that broke an invariant, with a deliberately broken pass both in the manager and through
  the generators (`TestVerifyEachPassNamesTheBreakingPass`).
- Print after pass: the dump hook is called once per pass in order, only the final generation prints, and a request to
  print without a writer is an error.
- Options: the fields of `gen.Go` and `gen.TypeScript` and the flags of `zp gen` reject an unknown profile, optimization
  and pass, accept `speed`, and change the output only through `NoOpt`.

## Performance and Results

The manager adds a constant number of checks to a sequence of passes that is linear in the grammar.
`gen.Go{Types: true}.Generate` and `gen.TypeScript{Types: true}.Generate` on the DuckDB grammar and
`gen.Go{Types: true}.Generate` on the Go grammar, run five times alternately with the commit before the pipeline on the
same machine (2 iterations each), did not change beyond noise. The medians of the five runs before and after were 2.47 s
and 2.46 s for DuckDB Go, 0.467 s and 0.468 s for DuckDB TypeScript, and 0.596 s and 0.598 s for the Go grammar;
allocations per operation differ by less than 0.002 percent.

The generated parsers of the checked-in grammars, and the generated source of 356 grammars under 18 option sets for Go
and TypeScript, are byte-identical to those of the former sequences.

`memory` and `size` are measured as described under [Profiles](#profiles) before they are registered or documented.

## Limitations and Open Questions

- **What `memory` and `size` contain.** Their flag sets are hypotheses until measured. Whether they also set the
  typed slab controls, and whether `size` also changes the runtime embedded in the parser, is open.
- **Granularity of `term-rewrites`.** Each rewrite (`TextEq`, `ConcatFused`, `ProjectedMap`) could have its own flag. No
  printer reads the rewritten terms, so a flag changes only the Core IR, and splitting it earns nothing until the Flow
  printer takes terms from Core.
- **Flow and Exec passes.** When Flow or Exec IR gains an optional pass, the manager runs it, and the flag and profile
  tables grow. Whether Flow lowering's options (`class-tables`, `inline-literals`) become passes is decided with them.
- **Observability and budget controls as passes.** They are modes, not optimizations, so they stay out of the flag set;
  generated parsers keep fixing them at generation time.
