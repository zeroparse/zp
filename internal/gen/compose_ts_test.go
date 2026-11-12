package gen_test

// tsCompose is TypeScript that the table scripts include after they import the generated module as p.
// Its functions call the compositions that generated parsers print from Exec IR (the term methods of
// the parser) as the lowered terms do: a list or a tuple pushes its elements on kidStack and ends with
// termHull, a struct sets its fields and covers their ranges, and an iteration reads the items and
// steps through them with termItemAt. A failure that a composition returns is thrown as FxError,
// which the scripts report.
const tsCompose = `
class FxError {
  msg: string;
  constructor(msg: string) {
    this.msg = msg;
  }
}
function fxRaise(e: p.Value): void {
  if (e !== null) throw new FxError(e as string);
}
function fxValue<T>([v, e]: [T, p.Value]): T {
  fxRaise(e);
  return v;
}
function fxPush(c: p.Actx, fn: string, v: p.Value): void {
  fxRaise(c.p.termPush(c, fn, v));
}
function fxList(c: p.Actx, ...args: p.Value[]): p.Value {
  const base = c.p.kidStack.length;
  c.spend();
  for (const a of args) {
    c.spend();
    fxPush(c, "list", a);
  }
  return c.listNode(c.p.kids(base));
}
function fxTuple(c: p.Actx, ...args: p.Value[]): p.Value {
  const base = c.p.kidStack.length;
  c.spend();
  for (const a of args) {
    c.spend();
    fxPush(c, "tuple", a);
  }
  return c.p.termHull(c, true, c.p.kids(base));
}
function fxStruct(c: p.Actx, typ: string, ...kv: p.Value[]): p.Value {
  const n = c.termStruct(typ);
  let first = true;
  for (let i = 0; i < kv.length; i += 2) {
    c.spend();
    const v = kv[i + 1]!;
    c.termSetField(n, kv[i] as string, v);
    first = c.p.termCover(n, v, first);
  }
  c.p.termMade(c, n, first);
  return n;
}
function fxMember(c: p.Actx, x: p.Value, name: string): p.Value {
  const kind = c.p.memberKind(name);
  return fxValue(c.p.termMember(c, kind, x, name, c.p.memberName(kind, name)));
}
function fxItems(c: p.Actx, fn: string, v: p.Value): readonly (p.Node | null)[] {
  return fxValue(c.p.termItems(fn, v));
}
function fxFold(c: p.Actx, fn: string, right: boolean, acc: p.Value, list: p.Value, f: (acc: p.Value, item: p.Value) => p.Value): p.Value {
  const items = fxItems(c, fn, list);
  for (let i = 0; i < items.length; i++) {
    c.spend();
    acc = f(acc, c.p.termItemAt(items, i, right));
  }
  return acc;
}
function fxMap(c: p.Actx, list: p.Value, f: (item: p.Value) => p.Value): p.Value {
  const base = c.p.kidStack.length;
  c.spend();
  const items = fxItems(c, "map", list);
  for (let i = 0; i < items.length; i++) {
    c.spend();
    fxPush(c, "map", f(c.p.termItemAt(items, i, false)));
  }
  return c.listNode(c.p.kids(base));
}
function fxFilter(c: p.Actx, list: p.Value, f: (item: p.Value) => p.Value): p.Value {
  const base = c.p.kidStack.length;
  const items = fxItems(c, "filter", list);
  for (let i = 0; i < items.length; i++) {
    c.spend();
    if (fxValue(c.p.termFilterKeep(f(c.p.termItemAt(items, i, false))))) {
      c.p.kidStack.push(items[i]!);
    }
  }
  return c.listNode(c.p.kids(base));
}
`
