#!/bin/sh
# Print the packages whose tests a change can affect.
#
#	hack/affected-tests.sh [base [head]]      packages, one import path per line (default base origin/main, head HEAD)
#	hack/affected-tests.sh -why [base [head]] each package with the changed file that selected it
#
# The set is the reverse closure, over imports, test imports and external test imports, of
#   - the packages that contain a changed file, or embed it;
#   - the packages whose Go files name the path of a changed non-Go file or one of its parent directories
#     (testdata/grammars, spec/language, docs, ...): tests read such files by path;
#   - the packages that build generated programs (HEAVY below), when a file of the runtime, of the generated
#     runtime or of the generator test helpers changed, because those programs reach it through a copy or a
#     replace directive that the import graph does not show.
# A change of go.mod, go.work, go.sum or this script selects every package. The output ends with the lines
# "# generate" and "# alpha" when `go generate ./gen ./bench` or hack/test-alpha-parsers.sh is affected.
set -eu
why=0
[ "${1:-}" = -why ] && { why=1; shift; }
base=${1:-origin/main}
head=${2:-HEAD}
root=$(git rev-parse --show-toplevel)
cd "$root"
git diff --name-only "$base" "$head" > "${TMPDIR:-/tmp}/affected.$$.files"
trap 'rm -f "${TMPDIR:-/tmp}/affected.$$.files"' EXIT
go list -json ./... | python3 -I -c '
import json, sys, os, re, subprocess
why = int(sys.argv[1]); files = [l.strip() for l in open(sys.argv[2]) if l.strip()]
root = os.getcwd(); mod = "zeroparse.dev/zp"
dec = json.JSONDecoder(); s = sys.stdin.read(); i = 0; pk = {}
while i < len(s):
    while i < len(s) and s[i].isspace(): i += 1
    if i >= len(s): break
    o, i = dec.raw_decode(s, i); pk[o["ImportPath"]] = o
rel = {p: os.path.relpath(o["Dir"], root) for p, o in pk.items()}
bydir = {r: p for p, r in rel.items()}
HEAVY = ["internal/gen", "internal/gen/gengo", "internal/gen/gents", "internal/engine", "runtime",
         "internal/complexitytest", "spec/language", "gen", "bench", "internal/compile"]
HEAVY_TRIG = ("runtime/", "gen/_runtime/", "internal/gentest/", "internal/gen/gents/tsrt/")
rev = {p: set() for p in pk}
for p, o in pk.items():
    for q in (o.get("Imports") or []) + (o.get("TestImports") or []) + (o.get("XTestImports") or []):
        if q in rev: rev[q].add(p)
text = {}
for p, o in pk.items():
    t = ""
    for f in sorted(os.listdir(o["Dir"])):
        if f.endswith(".go"):
            try: t += open(os.path.join(o["Dir"], f), errors="replace").read()
            except OSError: pass
    text[p] = t
embed = {}
for p, o in pk.items():
    for f in (o.get("EmbedFiles") or []) + (o.get("TestEmbedFiles") or []) + (o.get("XTestEmbedFiles") or []):
        embed.setdefault(os.path.relpath(os.path.join(o["Dir"], f), root), set()).add(p)
sel = {}
prop = set()
def add(p, f, up=False):
    sel.setdefault(p, f)
    if up: prop.add(p)
allp = False; gen = False; alpha = False
for f in files:
    if f in ("go.mod", "go.work", "go.sum", "hack/affected-tests.sh") or f.startswith("runtime/go."):
        allp = True; sel.update({p: f for p in pk}); continue
    d = os.path.dirname(f); own = None
    while d:
        if d in bydir: own = bydir[d]; break
        d = os.path.dirname(d)
    if own and f.endswith((".go", ".s")): add(own, f, True)
    for p in embed.get(f, ()): add(p, f, True)
    if own and not f.endswith(".go"): add(own, f)
    if not f.endswith(".go") or own is None:
        parts = f.split("/")
        for k in range(1, len(parts)):
            a = "/".join(parts[:k])
            if k == 1: continue
            for p, t in text.items():
                if a in t: add(p, f)
    if f.startswith(HEAVY_TRIG):
        for h in HEAVY: add(mod + ("/" + h if h else ""), f, True)
    if f.startswith(("gen/", "bench/", "internal/gen/", "internal/compile/", "internal/engine/", "internal/syntax/", "internal/ir/", "runtime/", "testdata/grammars/", "internal/bench/")):
        gen = True
    if f.startswith(("testdata/grammars/", "internal/gen/", "internal/compile/", "internal/engine/", "internal/syntax/", "runtime/", "gen/", "internal/ir/")):
        alpha = True
work = list(prop); seen = {}
while work:
    p = work.pop()
    for r in rev.get(p, ()):
        if r not in sel: sel[r] = sel[p]; work.append(r)
if allp: pass
for p in sorted(sel):
    print((p + "\t" + sel[p]) if why else p)
if gen: print("# generate")
if alpha: print("# alpha")
' "$why" "${TMPDIR:-/tmp}/affected.$$.files"
