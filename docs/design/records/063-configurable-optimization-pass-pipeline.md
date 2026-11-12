# 063. Configurable Optimization Pass Pipeline

- **Status**: Implemented
- **Author**: @ornew
- **Date**: 2026-10-25

## Summary

The passes over Core IR form a pipeline that a small manager runs, and every optimization is a named
flag in one registry:

- each pass declares its name, the analyses it requires and invalidates, its mode applicability and its obligation;
- a flag per optimization turns a pass or one of its variants off;
- an optional verifier runs between passes;
- a profile (`speed`, `memory` and `size`) is a preset: a name for fixed flag values, with no logic of its own;
- the module can be printed after any pass.

The default pipeline produces the generated output of the former hand-written sequences byte for byte. Users select a
profile with `zp gen -optimize`, set flags with `-opt`, remove optimizations with `-no-opt`, and print the Core IR with `-print-after`.

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
- Optimization profiles did not exist. The size and speed of generated parsers are decided by one fixed set of
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
- A profile is a preset of flag values; `speed` is the default.

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
- Changes to what a parser does by a profile. A profile changes generated code, never results, errors or events. A flag that changes the generated API or the representation of the results is possible (`ChangesOutput`), but it is always an opt-in and no profile sets it.
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

The four Core passes, declared:

| Pass | Requires | Invalidates | After | Applies when | Obligation |
|:--|:--|:--|:--|:--|:--|
| `terminals` | | `attributes` | | Typed table, lean, no observability | Same input consumed and expectations recorded as the expression it replaces |
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

The registry `internal/gen/opt` lists every optimization with a name, a kind (`Pass`, `Lowering`, `Printer`, `Runtime`), a
description, its value in the `speed` profile and whether it changes the output. The Core pipeline names the passes, and a test checks that the
registry has the name of every pass.

| Flag | Kind | Default | Controls | Cost |
|:--|:--|:--|:--|:--|
| `terminals` | pass | on | Value-free choices of many literals and classes run as one program (lean typed Go) | Data against code: the choice becomes a table in the parser |
| `text-dispatch` | printer | on | The Go and TypeScript printers emit a predicate that compares the text of a capture with literals as one switch | One switch against a chain of comparisons; none |
| `field-compare` | printer | on | The Go printer compares fields in the actions and predicates of typed rules without boxing string fields | None; fewer allocations |
| `guards` | pass | on | An alternative is skipped when the next character cannot start it | One read of the next character per choice, and 9% more source |
| `guarded-classes` | pass | on | The guarded-class marks that `guards` sets, and their lowering: a leading class consumes the character its guard read | None measured |
| `scans` | pass | on | A value-free repetition of a class or of any character runs as a loop | None measured |
| `class-runs` | pass | on | Consecutive classes of a value-free sequence run as one (lean typed Go) | None measured |
| `class-tables` | lowering | off | `flowlower.Options.ClassTableRanges`: the least number of ranges of a class that is matched through a table; 0 turns it off, and 5 is the value that was measured | Table data against code: 3% smaller source, about 1% slower; an opt-in |
| `inline-literals` | lowering | 4 | `flowlower.Options.InlineLiterals`: the longest literal, in code points, that is compared in place first; 0 turns it off | Code against calls: 7.5% more source |
| `method-sharing` | printer | on | Equal generated bodies share one helper | A call where a body was |
| `literal-sharing` | printer | on | Equal literals share one table (TypeScript also needs `ts-literal-sharing`) | An indirection to the table |
| `ts-literal-sharing` | printer | off | TypeScript literal tables are shared | A small cold parse about 1.5 times slower; an opt-in |
| `typed-object-slabs` | runtime | on | Typed objects are allocated in slabs | Memory: 6% more bytes and live heap |
| `typed-list-slabs` | runtime | on | Typed lists are allocated in slabs | Memory: 10% more bytes and live heap |
| `typed-output-bootstrap` | runtime | on | The first slab of the typed output is sized from the input | A few allocations before the slabs start |
| `sparse-memo` | runtime | on | The first-call table of large TypeScript parsers is sparse | A page lookup per check |

The guide has the same table with the measured costs; [record 136](../../optimizations/136-measured-effect-of-the-generation-flags.md)
has the measurements.

Everything is a flag. An optimization that makes a parser smaller, lighter on memory or faster, a representation choice
and a trade-off that a user may want are each one entry of the registry, with a kind, a default, a description and
the tests of its kind; there is no second mechanism next to it. A profile is only a preset of flag values and has no
logic that a flag does not.

A flag is a value: 0 or 1, or a count. A request sets a flag with `-opt name[=count]` (`GenOptions.Opt`) and removes
with `-no-opt`. The order of application is the profile, then `-opt`, then `-no-opt`, so `-no-opt` wins. An unknown name or a
value that the flag does not take is an error that lists the valid names.

A flag keeps the results, errors and events of the parser and changes its code and its resource use, unless the registry
marks it `ChangesOutput`: it changes the generated API or the representation of the results, as removing spans from
typed values would. Such a flag is an opt-in (the registry rejects one that is on in `speed`), no profile may set it
(a test), `-no-opt` on it does nothing, the flag-off parity tests and negative controls of the flags that keep behavior do
not apply to it, and it must name a contract test of its own that checks the changed contract. A flag can only
turn an optimization on where the pass allows it: turning it on never overrides `Applies`, so `class-runs` stays off in
a build with budget controls whatever the flag says, because the pass is unsound there. The soundness gate is the
pass's `Applies`, never a flag. A known name that does not apply to the parser being generated is accepted and does
nothing; an unknown name is an error that lists the valid ones. The mode facts of `flowlower.Options` (`Spend`, `Recovers`,
`Observe`) are not optimizations and have no flag, and neither does `DeadStores`, which the emitted code needs.

The prologue and rollback helpers of the generated Go (`Parser.Enter`, `Parser.Restore`) are not a flag: the generated
code always calls them and the Go compiler inlines them. Writing their statements out at each call site compiles to the
same machine code and parses within the noise of an identical copy (optimization 108), so it would only make the
generated source larger, and only an optimization that is effective gets a flag.

`guards` off implies `guarded-classes` off and makes `class-runs` convert every class run, which `After` allows.

### Verification Between Passes

With verification on (`GenOptions.VerifyEachPass`), the manager makes `attributes` valid and calls `core.Verify` after
every pass and names the pass that produced the first violation. Off (the default), it verifies once at the end.
Verification between passes can run more analyses than the default pipeline; the output does not change. The test
helpers of the generator set it, so every fixture of the suite runs with the check between passes.

### Profiles

A profile is a preset: a name for fixed flag values. `speed` is the default and sets every flag on, with `inline-literals` at 4, except the opt-ins `ts-literal-sharing` and `class-tables`, which are 0.
`-optimize` takes a profile name, and any other name is an error until the profile is registered.

A profile has to earn its place by a measurement. It is kept if it improves the metric it is named for by at least 1% beyond
an identical copy of the baseline, on at least six of the ten language grammars, and costs at most 3% of parse time as a
geometric mean with no grammar more than 6% slower; a profile that fails is not registered. The flags are held to the same
rule for `speed`: a flag is on in `speed` if it improves the parse time, or the other metric it exists for, by that rule
or costs nothing that was measured ([record 136](../../optimizations/136-measured-effect-of-the-generation-flags.md) has the numbers).

| Flag | `speed` | `memory` | `size` |
|:--|:--|:--|:--|
| `terminals` | on | on | on |
| `text-dispatch`, `field-compare` | on | on | on |
| `guards`, `guarded-classes` | on | on | on |
| `scans` | on | on | on |
| `class-runs` | on | on | on |
| `class-tables` | 0 | 0 | 5 |
| `inline-literals` | 4 | 4 | 0 |
| `method-sharing`, `literal-sharing` | on | on | on |
| `ts-literal-sharing` | 0 | 0 | 0 |
| `typed-object-slabs`, `typed-list-slabs` | on | off | on |
| `typed-output-bootstrap`, `sparse-memo` | on | on | on |

Against `speed`, as ratios of a geometric mean over the ten language grammars: `memory` allocates typed values one by one,
so bytes allocated are 0.886 and live heap 0.848 of `speed`'s, parse time is 0.996, and there are many more allocations
(4.06 times). `size` writes short literals as calls and matches large classes through tables: `parser.go` is 0.912 and the
stripped binary 0.986 of `speed`'s, for a parse time of 1.022. A third candidate that also turned off `guards`, `scans` and
`guarded-classes` made the source 15% smaller but parsing 14% slower (29% on one grammar) and was dropped. Adding a profile is a row
in the registry, a consumer of the profile name, and a measurement against the rule above.

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
`gen.TypeScript`, as `Types` mirrors `-types`: `Optimize string`, `Opt map[string]int`, `NoOpt []string`, `PrintAfter string` and
`PrintTo io.Writer`. A profile is applied first, then `-opt` sets flags, and `-no-opt` removes optimizations, so `-optimize size -no-opt
guards` is `size` without `guards`, and the same flag in `-opt` and `-no-opt` is off. `PrintAfter` without `PrintTo` is an error. The name `-optimize` avoids the rule
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
- Other routes: the same differential runs for the Node and recognizer parsers in four modes (plain, observability,
  limit controls, recognizer) and for TypeScript parsers in four modes (plain, typed, observability, limit controls),
  with a few flags off and with all of them off, and with the opt-in `ts-literal-sharing` on. Each route has a negative
  control that changes a literal in the generated code and requires the comparison to notice.
- Combinations: all optional flags off, and a seeded sample of pairs, with `Verify` after every pass. The full 2^n
  product is not run.
- Mode applicability: `class-runs`, `terminals` and guarded classes do not run in builds with work controls, Documents
  or (for `terminals`) observability, whatever the flags say.
- Manager: a pass that requires a stale analysis gets it recomputed; a pass that violates `After` fails validation;
  verification names the pass that broke an invariant, with a deliberately broken pass both in the manager and through
  the generators (`TestVerifyEachPassNamesTheBreakingPass`).
- Print after pass: the dump hook is called once per pass in order, only the final generation prints, and a request to
  print without a writer is an error.
- Output-changing flags: `TestRegistryIsValid` rejects a registry in which a `ChangesOutput` flag is on in `speed`.
  `TestOutputFlagsHaveContractTests` requires each such flag to name a contract test, and keeps it out of the flag-off
  parity tests. Profiles may set only flags that keep behavior.
- Profiles: each profile other than `speed` is compared with the engine on every route: lean typed Go, the Node parser
  (plain, with observability, with limit controls), the recognizer, the Document runner, `ParseStream`, and TypeScript in four
  modes (`TestProfilesParity`, `TestProfilesRouteParity`, `TestProfilesStreamParity`, `TestProfilesTSParity`), and each profile
  must change the generated code of some case. The Document route has a negative control.
- Options: the fields of `gen.Go` and `gen.TypeScript` and the flags of `zp gen` reject an unknown profile, optimization
  and pass, accept `speed`, apply `Opt` and then `NoOpt` in that order (`NoOpt` wins), and change the output only through them.

## Performance and Results

The manager adds a constant number of checks to a sequence of passes that is linear in the grammar.
`gen.Go{Types: true}.Generate` and `gen.TypeScript{Types: true}.Generate` on the DuckDB grammar and
`gen.Go{Types: true}.Generate` on the Go grammar, run five times alternately with the commit before the pipeline on the
same machine (2 iterations each), did not change beyond noise. The medians of the five runs before and after were 2.47 s
and 2.46 s for DuckDB Go, 0.467 s and 0.468 s for DuckDB TypeScript, and 0.596 s and 0.598 s for the Go grammar;
allocations per operation differ by less than 0.002 percent.

The generated parsers of the checked-in grammars, and the generated source of 356 grammars under 18 option sets for Go
and TypeScript, are byte-identical to those of the former sequences.

`memory` and `size` were measured as described under [Profiles](#profiles); the numbers are in [record 136](../../optimizations/136-measured-effect-of-the-generation-flags.md).

## Limitations and Open Questions

- **Where `size` stops.** `size` does not change the runtime embedded in the parser, and the TypeScript printers have no flag
  that it sets. A TypeScript profile would need a parse-time measurement, which the harness does not make yet.
- **Flow and Exec passes.** When Flow or Exec IR gains an optional pass, the manager runs it, and the flag and profile
  tables grow. Whether Flow lowering's options (`class-tables`, `inline-literals`) become passes is decided with them.
- **Observability and budget controls as passes.** They are modes, not optimizations, so they stay out of the flag set;
  generated parsers keep fixing them at generation time.
