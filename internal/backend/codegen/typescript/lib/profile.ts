// Rule profiling; execution history belongs to each invocation. A parser generated without
// observability rejects the profile options.

/** Where a call began: its position and binding level. Generated parsers have no line information, so line and col are 0. */
export interface CallSite { pos: number; line: number; col: number; level: number }
export interface SkipReasonCount { reason: SkipReason; count: number }

/** The cost of one rule. time and selfTime are milliseconds from a monotonic clock. */
export interface RuleProfile {
  rule: string;
  entered: number;
  calls: number;
  memoHits: number;
  matched: number;
  failed: number;
  evals: number;
  reusedElements: number;
  consumed: number;
  skipped: number;
  skipReasons: SkipReasonCount[];
  examined: number;
  wasted: number;
  longestFail: number;
  longestFailAt: CallSite;
  repeats: number;
  maxEvals: number;
  maxEvalsAt: CallSite;
  time: number;
  selfTime: number;
}

/** An owned copy of the counts of a Profile, rules sorted by name. Completed counters exclude unfinished calls. */
export interface ProfileSnapshot {
  parses: number;
  entered: number;
  calls: number;
  evals: number;
  memoHits: number;
  skipped: number;
  reusedElements: number;
  examined: number;
  time: number;
  rules: RuleProfile[];
}

export interface ProfileData { totals: ProfileSnapshot; rows: Map<string, RuleProfile> }
// Weak keys let a discarded sink be collected; values contain only scalar rows.
export const profileData = new WeakMap<Profile, ProfileData>();

/** Accumulates the cost of each rule over the parses it is given to, without retaining input, nodes or call history. */
export class Profile {
  constructor() {
    profileData.set(this, { totals: { parses: 0, entered: 0, calls: 0, evals: 0, memoHits: 0, skipped: 0, reusedElements: 0, examined: 0, time: 0, rules: [] }, rows: new Map() });
  }

  /** Returns independent rows and nested arrays, sorted for deterministic JSON. */
  snapshot(): ProfileSnapshot {
    const data = profileData.get(this)!;
    const rules = Array.from(data.rows.values(), row => ({ ...row,
      longestFailAt: { ...row.longestFailAt },
      maxEvalsAt: { ...row.maxEvalsAt },
      skipReasons: row.skipReasons.map(reason => ({ ...reason })).sort((a, b) => a.reason < b.reason ? -1 : a.reason > b.reason ? 1 : 0),
    })).sort((a, b) => a.rule < b.rule ? -1 : a.rule > b.rule ? 1 : 0);
    return { ...data.totals, rules };
  }
}

// Avoid a DOM or Node type dependency, and never read the clock when disabled.
export function resolveProfileClock(): (() => number) | Error {
  const host = (globalThis as { performance?: { now?: unknown } }).performance;
  const now = host?.now;
  if (typeof now !== "function") return new Error("profiling requires host performance.now()");
  return now.bind(host) as () => number;
}

// A call in progress: its row, when it began, the time excluded before it, and the time of its nested calls.
export interface ProfileFrame { row: RuleProfile; start: number; excluded: number; child: number }

// ProfileRecorder adds the events of one invocation to a Profile. The observation records each event
// before the trace functions receive it and resumes the recorder after they return, so that their
// time is not counted.
export class ProfileRecorder {
  private data: ProfileData | null;
  private clock: (() => number) | null;
  private scope: object | null;
  private lastTime: number | null = null;
  // excluded is the time of the trace functions so far, and paused when the last event was recorded.
  private excluded = 0;
  private paused: number | null = null;
  private readonly frames: ProfileFrame[] = [];
  // seen counts the evaluating calls by rule, position and level; open counts the calls in progress by rule.
  private readonly seen = new Map<string, Map<number, Map<number, number>>>();
  private readonly open = new Map<string, number>();

  constructor(profile: Profile, clock: () => number, scope: object) {
    this.data = profileData.get(profile)!;
    this.clock = clock;
    this.scope = scope;
  }

  close(): void {
    this.data = null;
    this.clock = null;
    this.scope = null;
    this.lastTime = null;
    this.paused = null;
    this.frames.length = 0;
    this.seen.clear();
    this.open.clear();
  }

  private now(): number {
    try {
      const value = this.clock!();
      if (!Number.isFinite(value)) throw new Error("profiling clock must return a finite number");
      if (this.lastTime !== null && value < this.lastTime) throw new Error("profiling clock must be monotonic");
      this.lastTime = value;
      return value;
    } catch (value) { throw new TraceFailure(value, this.scope!); }
  }

  private row(rule: string): RuleProfile {
    let row = this.data!.rows.get(rule);
    if (row === undefined) {
      row = { rule, entered: 0, calls: 0, memoHits: 0, matched: 0, failed: 0, evals: 0, reusedElements: 0, consumed: 0, skipped: 0, skipReasons: [],
        examined: 0, wasted: 0, longestFail: 0, longestFailAt: { pos: 0, line: 0, col: 0, level: 0 },
        repeats: 0, maxEvals: 0, maxEvalsAt: { pos: 0, line: 0, col: 0, level: 0 }, time: 0, selfTime: 0 };
      this.data!.rows.set(rule, row);
    }
    return row;
  }

  // record adds the event e; the time of the event is when record reads the clock.
  record(e: TraceEvent): void {
    const now = this.now();
    switch (e.kind) {
      case "enter": {
        const totals = this.data!.totals, row = this.row(e.rule);
        if (this.frames.length === 0) totals.parses++;
        totals.entered++;
        row.entered++;
        this.open.set(e.rule, (this.open.get(e.rule) ?? 0) + 1);
        this.frames.push({ row, start: now, excluded: this.excluded, child: 0 });
        break;
      }
      case "exit":
        this.exit(e, now);
        break;
      case "skipped":
        this.skipped(e);
        break;
    }
    this.paused = now;
  }

  // resume tells the recorder that the trace functions have handled the last event recorded.
  resume(): void {
    if (this.paused !== null) {
      this.excluded += this.now() - this.paused;
      this.paused = null;
    }
  }

  private exit(e: TraceEvent, now: number): void {
    const frame = this.frames.pop();
    if (frame === undefined) return;
    const row = frame.row, totals = this.data!.totals;
    const elapsed = now - frame.start - (this.excluded - frame.excluded);
    row.selfTime += elapsed - frame.child;
    const open = this.open.get(e.rule)! - 1;
    this.open.set(e.rule, open);
    if (open === 0) row.time += elapsed;
    const examined = Math.max(e.examined - e.pos, 0);
    const parent = this.frames[this.frames.length - 1];
    if (parent === undefined) {
      totals.time += elapsed;
      totals.examined += examined;
      totals.reusedElements += e.reusedElements;
    } else parent.child += elapsed;
    row.calls++; totals.calls++;
    row.evals += e.evals; totals.evals += e.evals;
    row.reusedElements += e.reusedElements;
    row.examined += examined;
    if (e.memo) { row.memoHits++; totals.memoHits++; }
    if (e.matched) { row.matched++; row.consumed += e.end - e.pos; }
    else {
      row.failed++;
      if (!e.memo) {
        row.wasted += examined;
        if (examined > row.longestFail) { row.longestFail = examined; row.longestFailAt = { pos: e.pos, line: 0, col: 0, level: e.level }; }
      }
    }
    if (!e.memo && e.evals > 0) {
      let positions = this.seen.get(e.rule);
      if (positions === undefined) { positions = new Map(); this.seen.set(e.rule, positions); }
      let levels = positions.get(e.pos);
      if (levels === undefined) { levels = new Map(); positions.set(e.pos, levels); }
      const n = (levels.get(e.level) ?? 0) + 1;
      levels.set(e.level, n);
      if (n > 1) row.repeats++;
      if (n > row.maxEvals) { row.maxEvals = n; row.maxEvalsAt = { pos: e.pos, line: 0, col: 0, level: e.level }; }
    }
  }

  private skipped(e: TraceEvent): void {
    const row = this.row(e.rule);
    this.data!.totals.skipped++;
    row.skipped++;
    let reason = row.skipReasons.find(x => x.reason === e.skipReason);
    if (reason === undefined) { reason = { reason: e.skipReason, count: 0 }; row.skipReasons.push(reason); }
    reason.count++;
  }
}
