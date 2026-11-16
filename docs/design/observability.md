# Observability

Tracing, profiling, explanations and compute limits are part of every backend: each one that runs a parse can report
what it did, through one model of events and profiles. This page explains that model, how it is shared across the
backends and what it costs. The user-facing contract is [Observability](../../spec/runtime/observability.md); the
architecture as a whole is in [Architecture](architecture.md).

> [!WARNING]
> The code is being migrated to the architecture this page describes and still differs from it in places. The package
> comments describe the code as it is.

## The Event and Profile Model

The model of events and profiles lives in the `instrument` package of the runtime. It holds the trace function with its
enter, exit and skipped events, the rule filter and the depth limit, and the per-rule profile (calls, time, exact
Examined and Wasted). The engine and the generated parsers implement it with the same types and the same meaning, and
the command-line tools `explain`, `trace` and `profile` are built on it.

## Shared Across Backends

- **The same meaning.** Observation never changes what the parser accepts or returns. A trace or a profile describes the
  parse that happened; it does not select another parse.
- **The cost when it is off.** A parser that is not observed pays a single predictable check per observation point.
- **Specialization, not a second code path.** Generated code gets the same features through specializations of the Exec
  stage for the features of a build, so the plain build stays free of them. The ordinary, budget-controlled, observed
  and Document variants of a mechanism are specializations of one composition.
- **Failures carry the same information.** A failure in a trace event carries the same syntax error that the parse
  returns, so a traced failure and a plain failure can be compared.
