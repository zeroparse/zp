# Tracing and Profiling

A parse can report every rule call it makes to a caller-supplied function
(**tracing**) and can accumulate the cost of each rule over many parses
(**profiling**). Observation never changes the result of a parse. This chapter
defines the engine API and the opt-in APIs of generated Go and TypeScript
parsers.

## Observation Options

The engine (package `engine`) and generated Go parsers select what a parse
reports with the same four parse options, which have the same meaning in both.
Generated Go parsers accept them only when generated with observability (see
[Generated Go Batch Tracing](#generated-go-batch-tracing)).

| Option | Meaning |
|:--|:--|
| `WithTrace(f)` | Calls `f` with every event of the parse that the trace filter and the depth limit select |
| `WithProfile(prof)` | Adds every call of the parse to the `Profile` `prof` |
| `WithTraceFilter(rules...)` | Selects the calls of the named rules and the calls nested in them |
| `WithTraceMaxDepth(n)` | Selects the calls at most `n` deep; the start rule has depth 1 |

Several `WithTrace` options compose: each function receives every selected
event, a copy of its own, in the order of the options, and a function receives
an event only after the functions before it have returned; if one panics, the
parse ends with that panic and the functions after it do not receive the event.
Several `WithProfile`
options compose too: each profile records the whole parse. A nil function or
profile is ignored.

`WithTraceFilter` and `WithTraceMaxDepth` select the events that the trace
functions receive; they apply to all of them. A call is selected by the filter
if its rule is one of the named rules or the call is nested in a selected call;
a skipped call (`TraceSkipped`) is selected in the same way, as if it were made
at its place. A name that is not a rule selects no call, and a filter with no
names selects every call. A call deeper than `n` is not selected; zero or a
negative `n` sets no limit. The enter and exit events of a call are both
selected or both not. If several `WithTraceFilter` or several `WithTraceMaxDepth`
options are given, the last of each applies. Neither option changes the parse,
the events' fields (`Depth` stays the depth in the whole parse) or what the
profiles record.

A `Document` keeps the options given when it is created as defaults for every
parse of the Document; the options given to one parse apply over those defaults
for that parse only. Trace functions and profiles given to a parse are added to
those of the defaults, so both receive the parse; a trace filter or depth limit
given to a parse replaces the default for that parse.

## Engine Tracing

`engine.WithTrace(f)` calls `f` at the start and at the end of every rule call of
the parse, in order, so that the events nest like the calls.
Tracing works with every backend, with `Parse`, `ParsePrefix`, recognition,
`ParseStream` and `Document`. The trace options of generated parsers are those of
their own package, with the same meaning
([Observation Options](#observation-options)). The events are the `TraceEvent`
of the package `core`, which every producer of events uses; `engine.TraceEvent`
names the same type.

The engine reports the calls that its backend makes, as defined in
[Reported Calls](#reported-calls). It never reports `TraceSkipped`.

A `TraceEvent` has the following fields.

| Field | Meaning |
|:--|:--|
| `Kind` | `TraceEnter`, reported when a rule is called before the memo is consulted, or `TraceExit`, reported when the call returns |
| `Rule` | The name of the rule that is called |
| `Level` | The binding level the rule was called with, or 0 for a call without a level |
| `Depth` | The nesting depth of the call; the start rule has depth 1 |
| `Pos` | The position at which the call starts, in the position unit |
| `Lookahead` | True if the call is made where failures record no expectations: inside a lookahead, or inside the `skip` expression of an Operators Rule, which is matched as in a lookahead. It has the same value on the call's exit |

An exit event also has the fields below.

| Field | Meaning |
|:--|:--|
| `Matched` | Whether the rule matched |
| `End` | The position after the match, or `Pos` if the rule failed |
| `Memo` | True if the result came from the memo table without evaluating the body |
| `Evals` | The number of times this call evaluated the rule body: 0 for a memo result, usually 1, and one more for each step that a left-recursive match grows. Bodies of nested calls are not included |
| `ReusedElements` | The number of repetition elements of a `Document` reused during this call, nested calls included. It measures resumption in this invocation, not work from the parse that built a memo result |
| `Examined` | The call's exact examined end, defined below |

`Examined` is the exact end h of the call's
[examined range](documents.md#reuse-model): one past the greatest slot that the
call, or a call within it, examined. Looking at a character to reject it counts,
and so does input examined inside a lookahead. It is at least the call's start
and its end, is one past the length of the input when the call tested the end of
the input, and never includes input examined by the caller before the call. A
memo hit reports the end stored with the result it reuses. A call that reuses a
growing left-recursive seed extends its end to the end stored with that seed, so
the `Examined` of a call depends on the seed it reuses as well as on its other
fields.

The methods `LineCol(pos)`, `Text()`, `Failure()` and `Recovered()` read the
state of the parser and MUST be called only from the trace function, while it
handles the event. `Text` returns the input the call matched. `Failure` returns
the farthest failure recorded during the call as a `*SyntaxError`, or nil.
`Recovered` returns the errors that `#recover` recovered from during the call,
which `Failure` does not include. `LineCol` returns 0, 0 for a position in input
that a stream parse has discarded. The fields can be kept after the call.
Tracing does not change the result.

## Reported Calls

Every producer of events, the engine with each backend and every generated
parser, reports the rule calls that it makes. A call that a producer does not
make because the next character cannot start the rule (first-character
dispatch, which [Evaluations of Rules](parsing.md#evaluations-of-rules)
limits) is not reported as a call; a generated parser MAY report it as
`TraceSkipped`. Producers do not skip the same calls, so the events of two
producers for the same parse, in the same position unit, agree on every call
that can match and MAY differ only in calls that cannot match: calls skipped by
dispatch, and calls that fail having examined no further than the character at
their position. On the calls that both make, every field of their events is
equal. The same holds for their profiles once the counts of those calls are
taken out. Document parses after an edit are the exception: the engine MAY
resume repetitions and reuse failed results of the previous parse, so the
producers report different calls. Of the calls with the same rule, position,
level, depth, end and outcome, if both producers make any, both make the same
number; they pair one to one in the order they are made, and paired calls have
the same `Examined`. A producer can make such a call more
than once with different `Examined`, within the growth of a left-recursive
seed: `Examined` depends on the seed that the call reuses, which those fields
leave out.

## Engine Profiling

`engine.WithProfile(prof)` adds the cost of a parse to `prof`, a `Profile` of
the package `core`, which accumulates any number of parses of any producer;
`engine.Profile` names the same type. The profiled parse is traced. A
`Profile` is not safe for concurrent use and keeps nothing of a parse after it
returns. A stream parse keeps the memory of the profile bounded: the profile
forgets the positions that the stream has discarded, where no rule is evaluated
again.

The engine records each event in the profile before the trace functions
receive it, so a snapshot taken by a trace function on an exit includes that
call, and the times exclude the time of the trace functions. Times include the
cost of measuring them and are comparable with each other only.

`Profile.Snapshot()` returns a `ProfileSnapshot`, an owned copy of the counts
with the rules sorted by name and each rule's skip reasons sorted by reason. It
has the totals `Parses`, `Entered`, `Calls`, `Evals`, `MemoHits`, `Skipped`,
`ReusedElements` (completed root calls only, counted once), `Examined` (over
completed root calls, the input from each call's position to its `Examined`),
`Time` (the time of the completed root calls) and `Rules`, a list of
`RuleProfile`. `ProfileSnapshot.Hints()` returns text that points at costly
rules. `Entered` counts calls that began, including calls interrupted before
their exit; the other counts and the times cover completed calls only. A
`RuleProfile` has the fields below.

| Field | Meaning |
|:--|:--|
| `Rule` | The rule name. Calls of a rule's value-free twin count as calls of the rule |
| `Entered` | The calls that began |
| `Calls`, `MemoHits`, `Matched`, `Failed` | Each completed call either evaluated the body or took the memoized result, and either matched or failed |
| `Evals` | Body evaluations: one per call not answered by the memo and one more for each growth step of a left-recursive match |
| `ReusedElements` | Repetition elements reused during the calls, nested calls included. A nested call can report the same reuse as its parent, so parent and child rows MUST NOT be summed |
| `Consumed` | The input matched by calls that matched |
| `Skipped`, `SkipReasons` | The calls reported as `TraceSkipped`, in all and by reason; always zero for the engine |
| `Examined` | The sum, over the calls, of `Examined - Pos` |
| `Wasted` | The same sum over the calls that evaluated the body and failed. It is inclusive of the calls they made |
| `LongestFail`, `LongestFailAt` | The most input one failed call that evaluated the body examined, and where |
| `Repeats` | The number of calls that evaluated the body at a position and level where an earlier call of the same parse had evaluated it |
| `MaxEvals`, `MaxEvalsAt` | The most evaluating calls at one position and level, and where |
| `Time`, `SelfTime` | The time spent in the calls, nested calls included (a call within a call of the same rule counts once), and excluding nested calls |

`LongestFailAt` and `MaxEvalsAt` are a `CallSite`: a `Location` (`Pos`, `Line`
and `Col`) with the `Level` of the call. Tied maxima keep the first location.
The line of a location is 0 when the producer has no line information, as for
generated parsers, or when the position lies in input that a stream parse has
discarded; such a location prints as `position N`. In JSON, the keys are the
camel-case field names, and `Time` and `SelfTime` are in nanoseconds.

## Generated Go Batch Tracing

Standalone generated Go Node parsers may be generated with tracing enabled by
`Observability` of `codegen.Go` or `zp gen -observability`. This mode
supports batch `Parse` and `ParseRule`; it also supports `Recognize` when that
entry point is generated. With generated types, it additionally supports
`ParseValue` and `ParseRuleValue`, for both direct typed execution and Node-conversion
fallback. Typed events describe rule calls made by the selected route;
conversion itself is not a rule event, so Node and typed event streams may
differ. Typed methods use the same event fields, filter, depth, interruption
and callback-panic behavior as Node batch methods. Generated Go Node and typed
batch parsers also provide profiling through `WithProfile`; TypeScript batch
parsers provide profiling through `Profile` in per-call options.

A parser generated without observability does not observe: passing it the trace or
profile options (`WithTrace`, `WithProfile`, `WithTraceFilter`, `WithTraceMaxDepth`)
returns an error before the parse instead of delivering partial data. A parser
generated with observability gives them the meaning that
[Observation Options](#observation-options) defines.

The generated package re-exports the event types of the package `core`
(`TraceEvent`, `TraceKind` with `TraceEnter`, `TraceExit` and `TraceSkipped`,
`SkipReason` and `SkipFirstCharacter`) and the trace options.
Events observe actual generated execution, not an event-for-event replay of
engine parser calls. `TraceEnter` begins a rule call before its memo lookup or
body dispatch; `TraceExit` reports the call's result and end position. The start
rule has depth 1. An exit describes the rule call only: whole-input acceptance
can still fail because input remains after the rule returns. `TraceSkipped` is
unpaired; it is emitted only for identifiable first-character dispatch skips,
with the `SkipReason` `SkipFirstCharacter` (`"first-character"`), and is not an
entered call. A call aborted by a runtime error or callback panic may have no
exit event.

An event has the fields of the engine's `TraceEvent` above, plus `SkipReason`
on a skipped event. Event positions use the selected position unit.
`Lookahead` is true for a call made where failures record no expectations:
inside a lookahead, or inside the `skip` expression of an Operators Rule, which
is matched as in a lookahead. It has the same value on the call's exit. `Memo=true` means the
call reused an existing memo result without evaluating its body, so `Evals` is
zero. Calls that evaluate left-recursion growth attempts have `Memo=false`;
their `Evals` counts their own body attempts, including growth attempts.
Recursive calls that reuse the growing seed have `Memo=true` and `Evals=0`.
Generated parsers do not resume repetitions, so `ReusedElements` is always 0.
Every exit event MUST report in `Examined` the call's exact examined end, as
[Engine Tracing](#engine-tracing) defines it.

Trace callbacks receive a copied event synchronously. Generated events carry no
parser state: their `LineCol` returns 0, 0, `Text` returns the empty string, and
`Failure` and `Recovered` return nil. In JSON an event has the keys `kind` (the
string `"enter"`, `"exit"` or `"skipped"`), `rule`, `level`, `depth`, `pos`,
`lookahead`, `matched`, `end`, `memo`, `evals`, `reusedElements`, `examined`
and `skipReason`. Callback panics propagate with their original value after
cleanup. Trace state is local to one invocation.

### Generated Go Batch Profiling

Generated Go Node and typed batch packages generated with
`Observability` MUST expose `WithProfile` and re-export from the package `core`
`Profile`, `ProfileSnapshot`, `RuleProfile`, `Location`, `CallSite` and
`SkipReasonCount`. A `Profile` is opaque. The zero value of `Profile` MUST be
ready to use; a nil profile is ignored. A profile sink is not
safe for concurrent use. Reentrant parses on one goroutine MUST keep separate
per-invocation accounting, and a profile MUST NOT retain anything of a parse
after it returns. `Snapshot()` MUST return an owned copy with rules sorted by
name and each rule's skip reasons sorted by reason; callers may marshal the
snapshot for deterministic JSON, whose keys are the camel-case field names.
`ProfileSnapshot.Hints()` returns text that points at costly rules.

The snapshot and its rows are those of [Engine Profiling](#engine-profiling).
The `Line` and `Col` of `LongestFailAt` and `MaxEvalsAt` are 0 because
generated events carry no line information.

`Parses` counts root invocations and `Entered` counts rule calls that began,
including calls interrupted before exit. `Calls`, `Evals`, `MemoHits`, match and
failure counts, consumed positions, and timing include only completed calls.
`Evals` counts body attempts, including left-recursive growth. `Repeats` and
`MaxEvals` count completed non-memo calls with `Evals > 0` at the same
`(rule, position, level)` within an invocation; tied maximum locations retain
the first observed location. `Skipped` and per-rule skip reasons count
identifiable generated dispatch skips independently of trace filters; skips
are not entered calls. Position counts use each invocation's configured unit
and MUST NOT be treated as normalized when a sink accumulates multiple units.

`Examined` and `Wasted` are defined with Document profiling below; a rule with
no completed call reports them as zero. `LongestFail` is the most input one
completed failed call with `Memo=false` examined. Root `Time` sums completed
root rule calls and excludes input setup, whole-input end checks and
typed-result conversion. Per-rule `Time` counts recursive calls of the same
rule once within each invocation, while `SelfTime` excludes child time. Both
include instrumentation and exclude the time of trace callbacks. A call is
added to the profile before the trace callback receives its exit, so a
snapshot taken inside that callback includes it. Callback panics MUST propagate
unchanged after cleanup. A panic in an exit callback MUST still count that
completed call; a panic in an enter callback does not synthesize an exit.

## Generated Go Document Tracing and Profiling

When generated Go observability and Document support are both enabled,
`NewDocument` MUST accept the observation options as retained defaults.
Construction MUST NOT emit parser events or profile counts. `Document.Parse`
and, with generated types, `Document.ParseValue` MUST accept them for one
invocation, combined with the defaults as
[Observation Options](#observation-options) defines; they MUST NOT change the
defaults for later calls. `Edit` does not parse and MUST NOT emit
rule events.

Events MUST report actual rule calls in the current invocation, including Node,
direct typed and Node-conversion fallback calls. Conversion itself MUST NOT
emit a rule event. A memoized whole-call reuse MUST report that call with
`Memo=true` and `Evals=0`; nested events from earlier parses MUST NOT be
replayed. Generated Documents do not resume repetition bodies, so their
`ReusedElements` is 0.

In a Document parse, an exit event's `Examined`, as
[Engine Tracing](#engine-tracing) defines it, is also the end against which a
later edit decides whether the call's memo result is reused. A call that
records a memo result MUST report the end stored with that result, and a memo
hit the end stored with the result it reuses. Results that reuse a growing
left-recursive seed are never reused after an edit.

A rule's profile `Examined` sums, over completed calls, `Examined - Pos`;
`Wasted` sums the same over completed failed calls with `Memo=false`. Batch and
Document parses of a rule accumulate into the same sums.

After a completed parse, the invocation's increases in profile `Evals` and
`MemoHits` MUST equal `Document.Stats().Evaluated` and `.Reused`, respectively.
`Stats` has no call counter. An
interrupted parse MAY leave statistics for calls that began but did not
complete. Temporary observer and profile state MUST be released after return
or panic; retained constructor defaults remain available for later calls.
While a Document generated with observability parses, its `Parse`,
`ParseValue` and `Edit` MUST return the generated `ErrDocumentBusy`, before
validating options and without changing the Document; this rejects calls a
callback makes on the same Document. The outer parse continues unaffected.
Callers SHOULD schedule later work or use another Document; parses of other
Documents and batch `Parse` calls remain allowed from callbacks.

## Generated TypeScript Batch Tracing

TypeScript batch tracing MUST be enabled at generation with
`Observability` of `codegen.TypeScript` or `zp gen -lang ts -observability`.
It applies to generated `parseWithOptions`, `parseRuleWithOptions` and, when
generated, `recognizeWithOptions`. With `-types`, the configured-start
`parseValueWithOptions` also accepts trace options; the positional-unit
APIs do not. TypeScript does not expose a named typed-rule entry point.

The tracing fields added to `ParseOptions` are:

```ts
trace?: (event: TraceEvent) => void;
traceRules?: readonly string[];
traceMaxDepth?: number;
```

`trace` is a synchronous callback receiving a copied `TraceEvent`;
`traceRules` selects named rules and their descendants, and `traceMaxDepth`
limits delivered events. Omitted or empty `traceRules` selects all rules;
unknown names select none. `traceMaxDepth` MUST be a safe integer. The start
rule has depth one; a nonpositive value means unlimited depth.
Callback mutation MUST NOT affect parser state or later events. If a callback
throws, the original thrown value MUST propagate after runtime cleanup, even
when it is not an `Error`; an aborted parse may omit exits for active calls.
Filtered events do not change matching. Typed-result conversion MUST NOT emit
a separate event; observed calls correspond to the selected parser path.

TypeScript events use camel-case fields. `kind` is `"enter"`, `"exit"` or
`"skipped"`; events also contain `rule`, `level`, `depth`, `pos`, `matched`,
`end`, `memo`, `evals`, `examined`, `examinedKnown` and `skipReason`. Positions
use the selected unit. `matched`, `end`, `memo` and `evals` describe exits;
`memo` indicates reuse without body evaluation, while `evals` counts body
evaluations. A skipped event is unpaired and is currently emitted only for
identifiable first-character dispatch skips. `examinedKnown` is always false
and `examined` MUST NOT be treated as a complete examined-input range. This
TypeScript batch API does not provide Document event tracing.

### Generated TypeScript Batch Profiling

TypeScript batch profiling MUST be emitted only when observability is enabled
with `Observability` of `codegen.TypeScript` or `zp gen -lang ts
-observability`. The generated module MUST export `Profile` and
`ProfileSnapshot`, and add `profile?: Profile | null` to `ParseOptions`.
`parseWithOptions`, `parseRuleWithOptions`, and `recognizeWithOptions` MUST
accept this option; with `-types`, `parseValueWithOptions` MUST accept it too.
Positional-unit APIs MUST NOT accept profiling options, and TypeScript
MUST NOT add a named typed-rule API for this feature.
Omitting the option or passing `null` MUST disable collection.

`Profile` MUST aggregate completed calls across invocations without retaining
input, callbacks, parser history or per-invocation frames. Each invocation MUST
keep independent accounting. A profile sink MUST NOT be shared by concurrent
calls. `snapshot()` MUST return independent rows and nested collections sorted
by rule name and skip reason, so JSON serialization is deterministic and
caller mutation cannot change the sink. Counters MUST use camel-case field
names corresponding to the generated Go profile. Entered-call counts MUST
include calls that began; completed-call counts and timings MUST exclude calls
without an exit. Profile collection MUST be independent of `traceRules` and
`traceMaxDepth`; skip counts MUST be independent of trace filters.
The snapshot's total fields are `parses`, `entered`, `calls`, `evals`,
`memoHits`, `skipped`, `time` and `rules`. Each rule row MUST expose `rule`,
`entered`, `calls`, `evals`, `memoHits`, `matched`, `failed`, `consumed`,
`skipped`, `skipReasons`, `repeats`, `maxEvals`, `maxEvalsAt`, `examined`,
`examinedKnown`, `wasted`, `wastedKnown`, `time` and `selfTime`.
Each `skipReasons` item MUST contain `reason` and `count`; `maxEvalsAt` MUST
contain `pos` and `level`. `repeats` MUST count additional completed non-memo
calls with `evals > 0` at the same `(rule, pos, level)` within an invocation.
`maxEvals` MUST be the largest such call count at a location, and ties MUST
retain the first observed location.

`examinedKnown` and `wastedKnown` MUST be false because examined and wasted
ranges are unavailable; implementations MUST NOT infer them. `time` and
`selfTime` MUST be milliseconds from the host's monotonic `performance.now()`
clock. Root time MUST exclude input setup, whole-input end checks and
typed-result conversion. Per-rule time MUST count recursive calls of the same
rule once per invocation; `selfTime` MUST exclude child time. Both MUST include
synchronous trace-callback work. When the exit-time clock read succeeds, a
completed call MUST be published after its exit callback returns or throws.
A snapshot inside that callback MAY omit the current exit until it unwinds.
An enter callback throw MUST NOT synthesize an exit.

Profiling MUST require `performance.now()` and MUST NOT fall back to
`Date.now()`. If the clock is unavailable, the profiled invocation MUST return
an `Error`. Valid clock readings MAY have coarse resolution and produce zero
durations. A thrown clock getter/call MUST preserve its original thrown value;
non-finite and backward readings MUST fail explicitly and MAY leave a partial
profile. If both a trace callback and a later clock read fail, the callback's
original thrown value MUST take precedence, and collection MAY omit that
exit. The implementation MUST NOT require DOM type declarations.
