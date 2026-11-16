// Rule tracing. A parser generated without observability rejects the trace options.

/** An actual rule entry, completed call, or identifiable dispatch skip. */
export type TraceKind = "enter" | "exit" | "skipped";

/** Why a call was not made: "first-character" for a first-character dispatch skip, "" otherwise. */
export type SkipReason = "" | "first-character";

/**
 * A rule call event, a copy that the trace function owns. Its fields and their JSON keys are those
 * of every producer of events; positions use the invocation's unit.
 */
export interface TraceEvent {
  readonly kind: TraceKind;
  readonly rule: string;
  readonly level: number;
  readonly depth: number;
  readonly pos: number;
  readonly lookahead: boolean;
  readonly matched: boolean;
  readonly end: number;
  readonly memo: boolean;
  readonly evals: number;
  readonly reusedElements: number;
  readonly examined: number;
  readonly skipReason: SkipReason;
}

/** A trace function. */
export type TraceFunction = (event: TraceEvent) => void;

export interface ObservationCall {
  readonly rule: string;
  readonly pos: number;
  readonly level: number;
  readonly depth: number;
  readonly lookahead: boolean;
  readonly selected: boolean;
  evals: number;
  memo: boolean;
}

// Remains wrapped until outside run and budget-control error classification.
export class TraceFailure {
  readonly value: unknown;
  readonly scope: object;
  constructor(value: unknown, scope: object) { this.value = value; this.scope = scope; }
}

// The observation of one invocation: it records each event in the profiles first, then hands the
// events that the trace filter and depth limit select to the trace functions, in order, each its own
// copy, so that the profiles do not count the time of the trace functions.
export class Observation {
  private traces: readonly TraceFunction[] | null;
  private recorders: ProfileRecorder[] | null;
  private scope: object | null;
  private parser: { readonly silent: number } | null;
  private rules: Set<string> | null;
  private readonly maxDepth: number;
  protected readonly stack: ObservationCall[] = [];

  constructor(o: ResolvedParseOptions, parser: { readonly silent: number }) {
    this.traces = o.traces;
    this.recorders = o.profiles.length === 0 ? null : o.profiles.map(profile => new ProfileRecorder(profile, o.profileClock!, o));
    this.scope = o;
    this.parser = parser;
    this.rules = o.traces.length === 0 ? new Set() : o.traceRules.length === 0 ? null : new Set(o.traceRules);
    this.maxDepth = o.traceMaxDepth;
  }

  release(): void {
    if (this.recorders !== null) for (const r of this.recorders) r.close();
    this.traces = null;
    this.recorders = null;
    this.scope = null;
    this.parser = null;
    this.rules = null;
    this.stack.length = 0;
  }

  private selected(rule: string): boolean {
    return this.rules === null || this.rules.has(rule) || (this.stack[this.stack.length - 1]?.selected ?? false);
  }

  private deliver(kind: TraceKind, call: ObservationCall, matched = false, end = 0, skipReason: SkipReason = ""): void {
    const recorders = this.recorders;
    const traced = call.selected && (this.maxDepth <= 0 || call.depth <= this.maxDepth);
    if (recorders === null && !traced) return;
    // Delivered objects never double as internal stack frames. A trace function may retain or
    // mutate its copy without changing later events, other functions' copies or call counts.
    const event: TraceEvent = {
      kind, rule: call.rule, level: call.level, depth: call.depth, pos: call.pos, lookahead: call.lookahead,
      matched, end, memo: call.memo, evals: call.evals, reusedElements: 0, examined: 0, skipReason,
    };
    if (recorders !== null) for (const r of recorders) r.record(event);
    if (!traced) return;
    let failed = false;
    try {
      for (const trace of this.traces!) trace({ ...event });
    } catch (value) {
      failed = true;
      throw new TraceFailure(value, this.scope!);
    } finally {
      // A failure of the clock must not replace what a trace function threw.
      try { if (recorders !== null) for (const r of recorders) r.resume(); }
      catch (value) { if (!failed) throw value; }
    }
  }

  enter(rule: string, pos: number, level: number): void {
    const call: ObservationCall = { rule, pos, level, depth: this.stack.length + 1, lookahead: this.parser!.silent > 0,
      selected: this.selected(rule), evals: 0, memo: false };
    this.stack.push(call);
    this.deliver("enter", call);
  }

  evaluated(): void {
    const call = this.stack[this.stack.length - 1];
    if (call !== undefined) call.evals++;
  }

  memoHit(): void {
    const call = this.stack[this.stack.length - 1];
    if (call !== undefined) call.memo = true;
  }

  exit(matched: boolean, end: number): void {
    this.deliver("exit", this.stack.pop()!, matched, end);
  }

  skipped(rule: string, pos: number, level: number): void {
    this.deliver("skipped", { rule, pos, level, depth: this.stack.length + 1, lookahead: this.parser!.silent > 0,
      selected: this.selected(rule), evals: 0, memo: false }, false, 0, "first-character");
  }
}

// This outer boundary must not expose callback values to withBudgetControl's
// private Fatal/SignalPanic catches. It also clears option-owned references.
export function withObservedOptions<T>(o: ResolvedParseOptions, body: (control: BudgetControl | null) => T, interrupted: (error: Error) => T): T {
  try {
    const value = withBudgetControl(o, body, interrupted);
    return value;
  } catch (x) {
    if (x instanceof TraceFailure && x.scope === o) throw x.value;
    throw x;
  } finally {
    o.profiles = noProfiles;
    o.profileClock = null;
    o.traces = noTraces;
    o.traceRules = [];
  }
}
