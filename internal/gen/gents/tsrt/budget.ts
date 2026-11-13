// Portable invocation controls are part of the generated runtime module.

/** A cooperative signal without a required DOM typing dependency. */
export interface CancellationSignal {
  readonly aborted: boolean;
  readonly reason?: unknown;
}

export type ParseInterruptionKind = "canceled" | "compute-limit" | "memory-limit" | "input-limit" | "depth-limit";

/** An interrupted invocation returns no partial Node or typed result. */
export class ParseInterruptionError extends Error {
  readonly kind: ParseInterruptionKind;
  readonly limit: number;
  readonly used: number;
  readonly requested: number;

  constructor(kind: ParseInterruptionKind, limit = 0, used = 0, requested = 0, cause: unknown = undefined) {
    super(kind === "canceled" ? "parse canceled" : kind === "input-limit" ? `input-limit exceeded: limit ${limit}, length ${requested}` : kind === "depth-limit" ? `nesting too deep: more than ${limit} nested recursive rule calls` : kind === "memory-limit" ? `memory-limit exceeded: limit ${limit}, used ${used}` : `${kind} exceeded: limit ${limit}, used ${used}, requested ${requested}`);
    this.name = "ParseInterruptionError";
    this.kind = kind;
    this.limit = limit;
    this.used = used;
    this.requested = requested;
    Object.defineProperty(this, "cause", { value: cause, configurable: true });
  }
}
// Interface merging exposes cause with ES2020 or later Error declarations.
export interface ParseInterruptionError { readonly cause: unknown }

export interface ResolvedParseOptions {
  profile: Profile | null;
  profileClock: (() => number) | null;
  trace: ((event: TraceEvent) => void) | null;
  traceRules: readonly string[];
  readonly traceMaxDepth: number;
  readonly unit: Unit;
  readonly maxDepth: number;
  readonly expectations: boolean;
  signal: CancellationSignal | null;
  readonly budget: number;
  readonly memory: number;
}

// Signal getters are caller code. Keep their thrown identity through the
// parser's Fatal and host-stack-overflow classification boundaries.
export class SignalPanic {
  readonly value: unknown;
  constructor(value: unknown) { this.value = value; }
}

export class BudgetControl {
  private signal: CancellationSignal | null;
  private readonly budget: number;
  private budgetUsed = 0;
  private sinceCheck = 0;
  private readonly memory: number;
  /** The bytes of storage the parse has taken into use at the sites that allocate it (Parser.alloc). */
  mem = 0;
  /** held returns the bytes the parser's working stacks hold now; the count takes its largest value at a poll. */
  held: (() => number) | null = null;
  private heldPeak = 0;

  constructor(o: ResolvedParseOptions) {
    this.signal = o.signal;
    this.budget = o.budget;
    this.memory = o.memory;
  }

  release(): void { this.signal = null; }

  // check is the poll: the signal first, then the memory count. Memory is counted where storage is
  // taken into use and checked here, not reserved before an allocation, so the count can pass the
  // limit by what the parse allocates between two polls.
  check(): void {
    const signal = this.signal;
    if (signal !== null) {
      let aborted: boolean;
      let reason: unknown;
      try {
        aborted = signal.aborted;
        if (aborted) reason = signal.reason;
      } catch (x) { throw new SignalPanic(x); }
      if (aborted) throw new Fatal(new ParseInterruptionError("canceled", 0, 0, 0, reason));
    }
    if (this.memory !== 0) {
      const used = this.memoryUsed();
      if (used > this.memory) throw new Fatal(new ParseInterruptionError("memory-limit", this.memory, used, 0));
    }
  }

  /** memoryUsed returns the count the memory limit checks: the allocated bytes and the peak of the working stacks. */
  memoryUsed(): number {
    if (this.held !== null) {
      const h = this.held();
      if (h > this.heldPeak) this.heldPeak = h;
    }
    return this.mem + this.heldPeak;
  }

  charge(n: number): void {
    if (n >= 256 - this.sinceCheck) {
      this.check();
      this.sinceCheck = 0;
    }
    if (this.budget !== 0 && n > this.budget - this.budgetUsed) {
      this.check();
      throw new Fatal(new ParseInterruptionError("compute-limit", this.budget, this.budgetUsed, n));
    }
    // Unlimited spend needs no cumulative number that could lose safe-integer precision.
    if (this.budget !== 0) this.budgetUsed += n;
    this.sinceCheck = n >= 256 ? 0 : this.sinceCheck + n;
  }

  // chargeUnits reserves n units as n charges of one unit do. A run that fits without reaching a poll
  // is reserved at once; otherwise each unit is, so that a failed run reports the unit that does not
  // fit: used is the limit and requested one.
  chargeUnits(n: number): void {
    if (n < 256 - this.sinceCheck && (this.budget === 0 || n <= this.budget - this.budgetUsed)) {
      if (this.budget !== 0) this.budgetUsed += n;
      this.sinceCheck += n;
      return;
    }
    for (; n > 0; n--) this.charge(1);
  }
}

// The memory limit counts bytes by a model of the objects as V8 lays them out on a 64-bit host
// without pointer compression, measured on Node.js 24 as the growth of the heap over 200000 live
// objects of each kind: an object takes 24 bytes plus 8 for each field; an array 32 bytes, and
// with elements a backing store of 16 bytes plus 8 for each; a typed array about 200 bytes plus its
// bytes; a Map 184 bytes, growing by about 28 for each entry; a string of up to 12 one-byte units
// copied out of another takes 16 bytes plus its units rounded up to 8 (one unit is cached, so 0),
// and a longer one a slice of 32 bytes. Two-byte strings and the allocator's rounding are not
// modelled: the count estimates the storage and is the same on every run of a parse.

/** objectBytes is the size of an object with n fields. */
export function objectBytes(n: number): number { return 24 + 8 * n; }
/** nodeBytes is the size of a Node (11 fields). */
export const nodeBytes = 24 + 8 * 11;
/** entryBytes is the size of a memo entry (12 fields). */
export const entryBytes = 24 + 8 * 12;
/** arrayBytes is the size of an array of n elements allocated at its length. */
export function arrayBytes(n: number): number { return n === 0 ? 32 : 48 + 8 * n; }
/** typedBytes is the size of a typed array of n bytes. */
export function typedBytes(n: number): number { return 200 + n; }
/** mapBytes is the size of an empty Map, and mapEntryBytes what each entry adds. */
export const mapBytes = 184;
export const mapEntryBytes = 28;
/** sliceBytes is the size of a string of n units copied or sliced out of another. */
export function sliceBytes(n: number): number { return n <= 1 ? 0 : n < 13 ? (16 + n + 7) & ~7 : 32; }
/** stringBytes is the size of a flat string of n units, as a joined string holds once read. */
export function stringBytes(n: number): number { return n <= 1 ? 0 : (16 + n + 7) & ~7; }

export function withBudgetControl<T>(o: ResolvedParseOptions, body: (control: BudgetControl | null) => T, interrupted: (error: Error) => T): T {
  if (o.signal === null && o.budget === 0 && o.memory === 0) return body(null);
  let control: BudgetControl | null = null;
  try {
    control = new BudgetControl(o);
    o.signal = null;
    control.check();
    const value = body(control);
    control.check();
    return value;
  } catch (x) {
    if (x instanceof SignalPanic) throw x.value;
    if (x instanceof Fatal) return interrupted(x.err);
    throw x;
  } finally {
    // Teardown must neither charge nor consult caller getters.
    control?.release();
    o.signal = null;
  }
}

export function utf8EncodingLenSpend(s: string, control: BudgetControl): number {
  let n = s.length;
  for (let i = 0; i < s.length; i++) {
    const c = stringUnitSpend(s, i, control);
    if (c < 0x80) {
      continue;
    }
    if (c < 0x800) {
      n++;
    } else if (c >= 0xd800 && c < 0xdc00 && i + 1 < s.length && (stringUnitSpend(s, i + 1, control) & 0xfc00) === 0xdc00) {
      n += 2; // a pair: 2 units, 4 bytes
      i++;
    } else {
      n += 2; // a lone surrogate is encoded as U+FFFD
    }
  }
  return n;
}


export function encodeUTF8Spend(s: string, control: BudgetControl): Uint8Array {
  const out = new Uint8Array(utf8EncodingLenSpend(s, control));
  control.mem += typedBytes(out.length);
  let j = 0;
  for (let i = 0; i < s.length; i++) {
    let c = stringUnitSpend(s, i, control);
    if (c < 0x80) {
      control.charge(1);
      out[j++] = c;
      continue;
    }
    if (c >= 0xd800 && c <= 0xdfff) {
      if (c < 0xdc00 && i + 1 < s.length && (stringUnitSpend(s, i + 1, control) & 0xfc00) === 0xdc00) {
        c = 0x10000 + ((c - 0xd800) << 10) + (stringUnitSpend(s, i + 1, control) - 0xdc00);
        i++;
      } else {
        c = 0xfffd;
      }
    }
    control.charge(c < 0x800 ? 2 : c < 0x10000 ? 3 : 4);
    if (c < 0x800) {
      out[j++] = 0xc0 | (c >> 6);
      out[j++] = 0x80 | (c & 0x3f);
    } else if (c < 0x10000) {
      out[j++] = 0xe0 | (c >> 12);
      out[j++] = 0x80 | ((c >> 6) & 0x3f);
      out[j++] = 0x80 | (c & 0x3f);
    } else {
      out[j++] = 0xf0 | (c >> 18);
      out[j++] = 0x80 | ((c >> 12) & 0x3f);
      out[j++] = 0x80 | ((c >> 6) & 0x3f);
      out[j++] = 0x80 | (c & 0x3f);
    }
  }
  return out;
}


export function runeCountSpend(s: string, control: BudgetControl): number {
  let n = s.length;
  for (let i = 0; i + 1 < s.length; i++) {
    const c = stringUnitSpend(s, i, control);
    if (c >= 0xd800 && c < 0xdc00 && (stringUnitSpend(s, i + 1, control) & 0xfc00) === 0xdc00) {
      n--;
      i++;
    }
  }
  return n;
}


export function wellFormedSpend(s: string, control: BudgetControl): string {
  const parts: string[] = [];
  let from = 0;
  for (let i = 0; i < s.length; i++) {
    const c = stringUnitSpend(s, i, control);
    if (c >= 0xd800 && c <= 0xdfff) {
      if (c < 0xdc00 && i + 1 < s.length && (stringUnitSpend(s, i + 1, control) & 0xfc00) === 0xdc00) {
        i++;
        continue;
      }
      parts.push(s.slice(from, i), replacementChar);
      from = i + 1;
    }
  }
  if (from === 0) {
    return s;
  }
  parts.push(s.slice(from));
  return joinStringsSpend(parts, control);
}


export function stringUnitSpend(s: string, i: number, control: BudgetControl): number {
  control.charge(1);
  return s.charCodeAt(i);
}
export function byteUnitSpend(b: Uint8Array, i: number, n: number, control: BudgetControl): number {
  control.charge(1);
  return i < n ? b[i]! : 0;
}
export function joinStringsSpend(parts: readonly string[], control: BudgetControl): string {
  for (const part of parts) {
    control.charge(1);
    for (let i = 0; i < part.length; i += 4096) control.charge(Math.min(4096, part.length - i));
  }
  const result = parts.join("");
  control.mem += stringBytes(result.length);
  control.check();
  return result;
}
export function codePointsStringSpend(cps: Int32Array, control: BudgetControl): string {
  const parts: string[] = [];
  for (let i = 0; i < cps.length; i += 4096) {
    const end = Math.min(i + 4096, cps.length);
    control.charge(end - i);
    parts.push(String.fromCodePoint(...cps.subarray(i, end)));
  }
  return joinStringsSpend(parts, control);
}

export function decodeRuneSpend(b: Uint8Array, i: number, n: number, control: BudgetControl): { rune: number; size: number } {
  let size = 1;
  const c0 = byteUnitSpend(b, i, n, control);
  if (c0 < 0x80) {
    return { rune: c0, size };
  }
  if (c0 < 0xc2 || c0 > 0xf4) {
    return { rune: 0xfffd, size };
  }
  const c1 = byteUnitSpend(b, i + 1, n, control);
  if (c0 < 0xe0) {
    if ((c1 & 0xc0) !== 0x80) {
      return { rune: 0xfffd, size };
    }
    size = 2;
    return { rune: ((c0 & 0x1f) << 6) | (c1 & 0x3f), size };
  }
  const lo = c0 === 0xe0 ? 0xa0 : c0 === 0xf0 ? 0x90 : 0x80;
  const hi = c0 === 0xed ? 0x9f : c0 === 0xf4 ? 0x8f : 0xbf;
  if (c1 < lo || c1 > hi) {
    return { rune: 0xfffd, size };
  }
  const c2 = byteUnitSpend(b, i + 2, n, control);
  if ((c2 & 0xc0) !== 0x80) {
    return { rune: 0xfffd, size };
  }
  if (c0 < 0xf0) {
    size = 3;
    return { rune: ((c0 & 0x0f) << 12) | ((c1 & 0x3f) << 6) | (c2 & 0x3f), size };
  }
  const c3 = byteUnitSpend(b, i + 3, n, control);
  if ((c3 & 0xc0) !== 0x80) {
    return { rune: 0xfffd, size };
  }
  size = 4;
  return { rune: ((c0 & 0x07) << 18) | ((c1 & 0x3f) << 12) | ((c2 & 0x3f) << 6) | (c3 & 0x3f), size };
}


export function equalStringsSpend(a: string, b: string, control: BudgetControl): boolean {
  if (a.length !== b.length) return false;
  for (let from = 0; from < a.length; from += 128) {
    const end = Math.min(from + 128, a.length);
    control.charge(end - from);
    if (a.slice(from, end) !== b.slice(from, end)) return false;
  }
  return true;
}

export function copyNodesSpend(items: readonly (Node | null)[], start: number, end: number, control: BudgetControl): (Node | null)[] {
  const out: (Node | null)[] = new Array(end - start);
  control.mem += arrayBytes(end - start);
  for (let from = start; from < end; from += 128) {
    const stop = Math.min(from + 128, end);
    control.charge(stop - from);
    for (let i = from; i < stop; i++) out[i - start] = items[i]!;
  }
  return out;
}

// These scalar helpers preserve surrogateescape bytes, unlike input encoding.
export function utf8LenSpend(s: string, control: BudgetControl): number {
  let n = s.length;
  for (let i = 0; i < s.length; i++) {
    const c = valueUnitSpend(s, i, control);
    if (c < 0x80) continue;
    if (c < 0x800) {
      n++;
    } else if (c >= 0xd800 && c < 0xdc00 && i + 1 < s.length && (valueUnitSpend(s, i + 1, control) & 0xfc00) === 0xdc00) {
      n += 2;
      i++;
    } else if (c < 0xdc80 || c > 0xdcff) {
      n += 2;
    }
  }
  return n;
}

export function isByteSpend(s: string, i: number, control: BudgetControl): boolean {
  const c = valueUnitSpend(s, i, control);
  return c >= 0xdc80 && c <= 0xdcff && !(i > 0 && (valueUnitSpend(s, i - 1, control) & 0xfc00) === 0xd800);
}

// Stream only the suffix bytes actually compared; keep byte escapes raw.
export function* utf8ValueBytesSpend(s: string, start: number, control: BudgetControl) {
  for (let i = start; i < s.length; i++) {
    let c = valueUnitSpend(s, i, control);
    if (c >= 0xd800 && c < 0xdc00 && i + 1 < s.length) {
      const low = valueUnitSpend(s, i + 1, control);
      if ((low & 0xfc00) === 0xdc00) {
        c = 0x10000 + ((c - 0xd800) << 10) + (low - 0xdc00);
        i++;
      }
    }
    if (c >= 0xdc80 && c <= 0xdcff) {
      control.charge(1);
      yield c & 0xff;
      continue;
    }
    if (c >= 0xd800 && c <= 0xdfff) c = 0xfffd;
    control.charge(c < 0x80 ? 1 : c < 0x800 ? 2 : c < 0x10000 ? 3 : 4);
    if (c < 0x80) {
      yield c;
    } else if (c < 0x800) {
      yield 0xc0 | (c >> 6); yield 0x80 | (c & 0x3f);
    } else if (c < 0x10000) {
      yield 0xe0 | (c >> 12); yield 0x80 | ((c >> 6) & 0x3f); yield 0x80 | (c & 0x3f);
    } else {
      yield 0xf0 | (c >> 18); yield 0x80 | ((c >> 12) & 0x3f); yield 0x80 | ((c >> 6) & 0x3f); yield 0x80 | (c & 0x3f);
    }
  }
}

export function compareStringsSpend(a: string, b: string, control: BudgetControl): number {
  const n = Math.min(a.length, b.length);
  for (let i = 0; i < n; i++) {
    let x = valueUnitSpend(a, i, control);
    let y = valueUnitSpend(b, i, control);
    if (x !== y) {
      if (isByteSpend(a, i, control) || isByteSpend(b, i, control)) {
        const p = utf8ValueBytesSpend(a, i, control);
        const q = utf8ValueBytesSpend(b, i, control);
        for (;;) {
          control.charge(2);
          const x = p.next();
          const y = q.next();
          if (x.done || y.done) return x.done === y.done ? 0 : x.done ? -1 : 1;
          if (x.value !== y.value) return x.value < y.value ? -1 : 1;
        }
      }
      if (x >= 0xd800 && y >= 0xd800) {
        x = x >= 0xe000 ? x - 0x800 : x + 0x2000;
        y = y >= 0xe000 ? y - 0x800 : y + 0x2000;
      }
      return x < y ? -1 : 1;
    }
  }
  return a.length < b.length ? -1 : a.length > b.length ? 1 : 0;
}

export function valueUnitSpend(s: string, i: number, control: BudgetControl): number {
  control.charge(1);
  return s.charCodeAt(i);
}

export function runeCountValueSpend(s: string, control: BudgetControl): number {
  let n = s.length;
  for (let i = 0; i + 1 < s.length; i++) {
    const c = valueUnitSpend(s, i, control);
    if (c >= 0xd800 && c < 0xdc00 && (valueUnitSpend(s, i + 1, control) & 0xfc00) === 0xdc00) {
      n--;
      i++;
    }
  }
  return n;
}

export function joinValueStringsSpend(parts: readonly string[], control: BudgetControl): string {
  for (const part of parts) {
    control.charge(1);
    for (let i = 0; i < part.length; i += 128) control.charge(Math.min(128, part.length - i));
  }
  const result = parts.join("");
  control.mem += stringBytes(result.length);
  control.check();
  return result;
}

// Public Node.field stays a caller operation; parsing/conversion share this
// internal visit counter and bounded string-name comparison.
export function getFieldSpend(n: Node, name: string, control: BudgetControl | null): Value {
  return findFieldSpend(n, name, control) ?? null;
}

// findFieldSpend is getFieldSpend that returns undefined for a field that the node does not have.
export function findFieldSpend(n: Node, name: string, control: BudgetControl | null): Value | undefined {
  for (const field of n.fields) {
    if (control === null) {
      if (field.name === name) return field.value;
      continue;
    }
    control.charge(1);
    if (equalStringsSpend(field.name, name, control)) return field.value;
  }
  return undefined;
}

// Reserve bounded logical visits before native hashing, which cannot be polled.
export function stringKeySpend(s: string, control: BudgetControl): string {
  for (let i = 0; i < s.length; i += 128) control.charge(Math.min(128, s.length - i));
  return s;
}

export function equalValuesSpend(a: Value, b: Value, control: BudgetControl): boolean {
  if (typeof a === "string" && typeof b === "string") return equalStringsSpend(a, b, control);
  return a === b;
}

export function copyEntriesSpend(items: readonly RecEntry[], start: number, end: number, control: BudgetControl): RecEntry[] {
  const out: RecEntry[] = new Array(end - start);
  control.mem += arrayBytes(end - start);
  for (let from = start; from < end; from += 128) {
    const stop = Math.min(from + 128, end);
    control.charge(stop - from);
    for (let i = from; i < stop; i++) out[i - start] = items[i]!;
  }
  return out;
}

export function copyIDsSpend(items: readonly number[], start: number, end: number, control: BudgetControl): number[] {
  const out: number[] = new Array(end - start);
  control.mem += arrayBytes(end - start);
  for (let from = start; from < end; from += 128) {
    const stop = Math.min(from + 128, end);
    control.charge(stop - from);
    for (let i = from; i < stop; i++) out[i - start] = items[i]!;
  }
  return out;
}

// Only newly collected, parser-owned expected strings may be sorted here.
export function sortExpectedSpend(values: string[], control: BudgetControl): void {
  const n = values.length;
  if (n < 2) return;
  control.charge(n);
  let src = values, dst = new Array<string>(n);
  for (let width = 1; width < n;) {
    for (let lo = 0; lo < n;) {
      const mid = Math.min(lo + width, n), hi = Math.min(mid + width, n);
      let i = lo, j = mid, k = lo;
      while (i < mid && j < hi) {
        const order = expectedKind(src[i]!) - expectedKind(src[j]!) || compareStringsSpend(src[i]!, src[j]!, control);
        control.charge(1);
        dst[k++] = order <= 0 ? src[i++]! : src[j++]!;
      }
      while (i < mid) { control.charge(1); dst[k++] = src[i++]!; }
      while (j < hi) { control.charge(1); dst[k++] = src[j++]!; }
      lo = hi;
    }
    const old = src; src = dst; dst = old;
    if (width >= n - width) break;
    width *= 2;
  }
  if (src !== values) for (let i = 0; i < n; i++) { control.charge(1); values[i] = src[i]!; }
}

export function reserveDiagnosticTextSpend(text: string, control: BudgetControl): void {
  for (let i = 0; i < text.length; i += 128) control.charge(Math.min(128, text.length - i));
}

// Reserve engine-owned text before the native Error constructor joins it.
// Native joining/allocation remain non-preemptible; public constructors and
// post-return reason() calls keep their existing caller-owned behavior.
export function reserveSyntaxMessageSpend(line: number, col: number, expected: readonly string[], messages: readonly string[], control: BudgetControl): void {
  reserveDiagnosticTextSpend(`${line}:${col}: `, control);
  const parts = messages.length !== 0 ? messages : expected;
  const shown = messages.length !== 0 ? parts.length : Math.min(parts.length, expectedInMessage);
  if (messages.length === 0) reserveDiagnosticTextSpend(expected.length !== 0 ? "syntax error: expected " : "syntax error", control);
  for (let i = 0; i < shown; i++) {
    control.charge(1);
    if (i !== 0) control.charge(2);
    reserveDiagnosticTextSpend(parts[i]!, control);
  }
  if (shown < parts.length) reserveDiagnosticTextSpend(expectedTail(parts.length - shown), control);
}

export function reserveRecoveredMessageSpend(errors: readonly SyntaxError[], control: BudgetControl): void {
  for (let i = 0; i < errors.length; i++) {
    control.charge(1);
    if (i !== 0) control.charge(1);
    reserveDiagnosticTextSpend(errors[i]!.message, control);
  }
}
