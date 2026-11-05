package core

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/ops"
)

// chargeRoots are the methods of the budget control that reserve compute units.
var chargeRoots = []string{"Charge", "Take", "takeSlow", "chargeStep", "chargeSlow"}

// runtimeCalls type-checks the Go runtime and returns its static call graph (calls through
// interfaces and function values are not edges) and the functions that reach a charge.
func runtimeCalls(t *testing.T) (*types.Package, map[*types.Func][]*types.Func, map[*types.Func]bool) {
	t.Helper()
	bp, err := build.Default.ImportDir(filepath.Join("..", "..", "..", "..", "runtime"), 0)
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var files []*ast.File
	for _, name := range bp.GoFiles {
		f, err := parser.ParseFile(fset, filepath.Join(bp.Dir, name), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}
	info := &types.Info{Uses: map[*ast.Ident]types.Object{}, Defs: map[*ast.Ident]types.Object{}}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	pkg, err := conf.Check("zeroparse.dev/zp/runtime", fset, files, info)
	if err != nil {
		t.Fatal(err)
	}
	callees := map[*types.Func][]*types.Func{}
	for _, f := range files {
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			caller := info.Defs[fd.Name].(*types.Func)
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				var id *ast.Ident
				switch fn := ast.Unparen(call.Fun).(type) {
				case *ast.Ident:
					id = fn
				case *ast.SelectorExpr:
					id = fn.Sel
				case *ast.IndexExpr: // an instantiated generic function
					switch g := fn.X.(type) {
					case *ast.Ident:
						id = g
					case *ast.SelectorExpr:
						id = g.Sel
					}
				}
				if id == nil {
					return true
				}
				if callee, ok := info.Uses[id].(*types.Func); ok && callee.Pkg() == pkg {
					callees[caller] = append(callees[caller], callee.Origin())
				}
				return true
			})
		}
	}
	charges := map[*types.Func]bool{}
	control := pkg.Scope().Lookup("BudgetControl").Type()
	for _, name := range chargeRoots {
		obj, _, _ := types.LookupFieldOrMethod(types.NewPointer(control), true, pkg, name)
		charges[obj.(*types.Func)] = true
	}
	for changed := true; changed; {
		changed = false
		for caller, cs := range callees {
			if charges[caller] {
				continue
			}
			for _, c := range cs {
				if charges[c] {
					charges[caller] = true
					changed = true
					break
				}
			}
		}
	}
	return pkg, callees, charges
}

// goCallee matches the calls in the Go form of an operation: a method of the parser (p.X), of one
// of its fields (p.A.X), or a function of the runtime.
var goCallee = regexp.MustCompile(`(?:\bp((?:\.[A-Za-z_]\w*)+)|(?:^|[^.\w])([A-Za-z_]\w*))\(`)

// opCallees returns the functions of the runtime that the Go form of the operation op calls, in the
// node or the typed runtime, or ok false when the target has no form of op.
func opCallees(t *testing.T, pkg *types.Package, op *exec.Op, typed bool) (form string, fns []*types.Func, ok bool) {
	t.Helper()
	tg := goTarget(goSpec{typed: typed})
	args := make([]string, len(op.Params))
	for i := range args {
		args[i] = fmt.Sprintf("a%d", i)
	}
	dst := make([]string, len(op.Results))
	for i := range dst {
		dst[i] = fmt.Sprintf("r%d", i)
	}
	c := &exec.Call{Callee: op.Name}
	func() {
		defer func() {
			if recover() != nil {
				form = ""
			}
		}()
		if s, done := tg.Stmt(c, args, dst, true); done {
			form = s
			return
		}
		form = tg.Op(c, args)
	}()
	if form == "" {
		return "", nil, false
	}
	recv := "Parser"
	if typed {
		recv = "TParser"
	}
	for _, m := range goCallee.FindAllStringSubmatch(form, -1) {
		if m[1] != "" {
			// Follow the fields of the parser to the method.
			var typ types.Type = types.NewPointer(pkg.Scope().Lookup(recv).Type())
			names := strings.Split(m[1][1:], ".")
			for i, name := range names {
				obj, _, _ := types.LookupFieldOrMethod(typ, true, pkg, name)
				switch o := obj.(type) {
				case *types.Var:
					typ = o.Type()
					continue
				case *types.Func:
					if i == len(names)-1 {
						fns = append(fns, o.Origin())
					}
				}
				break
			}
			continue
		}
		if f, ok := pkg.Scope().Lookup(m[2]).(*types.Func); ok {
			fns = append(fns, f.Origin())
		}
	}
	return form, fns, true
}

// chargeWrappers are the functions of the runtime that only pass charges on to the budget control.
var chargeWrappers = []string{"Spend", "SpendRead", "spendReadTracked", "spendValue", "growthSpend", "appendSpend", "copySpend", "clearSpend"}

// formSite stands for a charge that the Go form of an operation makes itself, through a wrapper.
const formSite = "the form"

// chargeSites returns the charge sites reachable from fns: the functions that call a wrapper or a
// method of chargeRoots, with formSite when fns holds a wrapper.
func chargeSites(pkg *types.Package, callees map[*types.Func][]*types.Func, charges map[*types.Func]bool, fns []*types.Func) map[string]bool {
	leaf := map[string]bool{}
	for _, n := range append(append([]string{}, chargeWrappers...), chargeRoots...) {
		leaf[n] = true
	}
	isLeaf := func(f *types.Func) bool { return leaf[f.Name()] && f.Pkg() == pkg }
	sites := map[string]bool{}
	seen := map[*types.Func]bool{}
	var walk func(f *types.Func)
	walk = func(f *types.Func) {
		if seen[f] || !charges[f] {
			return
		}
		seen[f] = true
		for _, c := range callees[f] {
			if isLeaf(c) {
				sites[siteName(f)] = true
			} else {
				walk(c)
			}
		}
	}
	for _, f := range fns {
		if isLeaf(f) {
			sites[formSite] = true
		} else {
			walk(f)
		}
	}
	return sites
}

// siteName is the name of a function of the runtime in siteCost: Type.Method or Function.
func siteName(f *types.Func) string {
	if r := f.Signature().Recv(); r != nil {
		t := r.Type()
		if p, ok := t.(*types.Pointer); ok {
			t = p.Elem()
		}
		return t.(*types.Named).Obj().Name() + "." + f.Name()
	}
	return f.Name()
}

// siteCost attributes each charge site that a transitional or host operation reaches to the quantities of
// its cost declaration that account for it; "" stands for the constant. An operation that reaches a
// site declares one of the site's quantities. A site that is not listed fails the test, so that a
// new charge in the runtime is attributed when it is added.
var siteCost = map[string][]string{
	formSite: nil, // any declared cost
	// Deferred memoization (seen.first).
	"Parser.controlledFirstCallPrepare": {"", ops.QPageIndex, ops.QPages, ops.QSeenIndex},
	"Parser.chargeValues":               {ops.QPageIndex, ops.QPages, ops.QSeenIndex},
	// The variable environment.
	"Parser.controlledEnvValues": {ops.QVars},
	"Parser.variable":            {ops.QEnvReads},
	// Recovered errors and expectations.
	"Parser.copyErrorsSlow": {ops.QErrs},
	"Parser.Expect":         {ops.QMerge},
	"Parser.MergeExpected":  {ops.QMerge},
	"Parser.Keep":           {ops.QMerge},
	// Left-recursion contexts and growths.
	"Parser.addDep":                {ops.QDeps, ""}, // "": ctx.enter adds one, the seed it reads
	"Parser.stored":                {ops.QDeps},
	"Parser.ctxFindSet":            {ops.QTable},
	"Parser.ctxStore":              {ops.QTable},
	"Parser.findJump":              {ops.QTable},
	"Parser.memoKeyEqual":          {ops.QTable},
	"Parser.compareTextControlled": {ops.QTable},
	"Parser.recordJumps":           {ops.QRun},
	"Parser.jumpShadowed":          {ops.QRun},
	"Parser.unionExpected":         {ops.QRun},
	"Parser.growBegin":             {""},
	"Parser.growStep":              {""},
	"Parser.canonicalSeed":         {""},
	"Parser.nodesEqual":            {""},
	"Parser.valuesEqual":           {""},
	"Parser.advance":               {ops.QSteps, ""},
	// The moves of the values of memo entries after Document edits.
	"Parser.NewNode":    {ops.QMoved},
	"Parser.movable":    {ops.QMoved},
	"Parser.moveNode":   {ops.QMoved},
	"Parser.moveShared": {ops.QMoved},
	// The stream window.
	"Parser.streamDiscard": {ops.QStreamScan},
	"Parser.streamLineCol": {ops.QStreamScan},

	// The capture frames.
	"Parser.undoCaptures":  {ops.QTrail},
	"TParser.undoCaptures": {ops.QTrail},
}

// chargedElsewhere are the operations whose declared cost comes from the charges of a target other
// than Go: the Go runtime charges nothing for them.
var chargedElsewhere = map[string]string{
	"frame.open": "the TypeScript runtime charges one unit for each slot of a new frame",
}

// runsGrammar are the transitional operations that run a rule body. Their charges include those of
// the grammar code they run, which a bound of the operation cannot state; the compositions that
// replace them (rule completion, the Document call) charge their own steps.
var runsGrammar = map[string]bool{"rule.invoke": true, "rule.invokePlain": true, "rule.run": true, "call.track": true, "call.trackBody": true,
	// The grammar code of an Operators Rule, and the actions that make the values of its lines.
	"opr.skip": true, "opr.match": true, "opr.lineValue": true, "opr.build": true, "opr.buildTyped": true}

// TestTransitionalAndHostCostsMatchCharges checks the cost declarations of the transitional and host
// operations against the Go runtime that implements them, through its static calls (calls through interfaces
// and function values are not followed, so the cost of a host callback such as the delivery of an
// element is not audited: it is the host's): an operation that reaches a charge of the budget may end
// the parse (NoReturn) and declares a cost with a quantity for every charge site it reaches
// (siteCost), and one that reaches none declares no cost. It does not measure the bounds: a
// transitional or host operation has no reference model to compute its quantities.
func TestTransitionalAndHostCostsMatchCharges(t *testing.T) {
	pkg, callees, charges := runtimeCalls(t)
	for _, op := range Build().Ops {
		if !op.Transitional && !op.Host || runsGrammar[op.Name] {
			continue
		}
		found := false
		var fns []*types.Func
		for _, typed := range []bool{false, true} {
			_, f, ok := opCallees(t, pkg, op, typed)
			found = found || ok
			fns = append(fns, f...)
		}
		if !found {
			if op.Host {
				t.Errorf("host operation %s has no Go form to audit", op.Name)
			}
			continue
		}
		sites := chargeSites(pkg, callees, charges, fns)
		declared := map[string]bool{}
		if op.Cost.Const != 0 {
			declared[""] = true
		}
		for _, term := range op.Cost.Terms {
			declared[term.Quantity] = true
		}
		if len(sites) == 0 {
			if len(declared) != 0 && chargedElsewhere[op.Name] == "" {
				t.Errorf("%s declares cost %s but reaches no charge", op.Name, op.Cost)
			}
			continue
		}
		if !op.NoReturn {
			t.Errorf("%s reaches a charge but is not NoReturn", op.Name)
		}
		if len(declared) == 0 {
			t.Errorf("%s reaches a charge but declares no cost", op.Name)
			continue
		}
		var names []string
		for s := range sites {
			names = append(names, s)
		}
		sort.Strings(names)
		for _, s := range names {
			qs, ok := siteCost[s]
			if !ok {
				t.Errorf("%s reaches the charge site %s, which siteCost does not attribute", op.Name, s)
				continue
			}
			if qs == nil {
				continue
			}
			if !slices.ContainsFunc(qs, func(q string) bool { return declared[q] }) {
				t.Errorf("%s reaches the charge site %s but its cost %s has none of %q", op.Name, s, op.Cost, qs)
			}
		}
	}
}
