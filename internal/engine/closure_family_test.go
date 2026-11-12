//go:build !zp_budget_controls_reference

package engine

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/cancel"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/engine/closure"
	"zeroparse.dev/zp/internal/engine/core"
	"zeroparse.dev/zp/internal/gentest"
	ir "zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/limits"
	"zeroparse.dev/zp/internal/value"
)

// leastCompute returns the smallest compute limit under which run succeeds.
func leastCompute(t *testing.T, run func(limits.Limits) error) uint64 {
	t.Helper()
	fits := func(b uint64) bool { return run(limits.Limits{Compute: b}) == nil }
	hi := uint64(1)
	for !fits(hi) {
		if hi > 1<<30 {
			t.Fatalf("the parse fails under every limit: %v", run(limits.Limits{}))
		}
		hi *= 2
	}
	lo := hi / 2
	for lo+1 < hi {
		mid := (lo + hi) / 2
		if fits(mid) {
			hi = mid
		} else {
			lo = mid
		}
	}
	return hi
}

// closureCharges returns the least compute limit of every kind of closure parse of input by prog,
// untraced and traced: each kind runs the closure family that its options select.
func closureCharges(t *testing.T, prog *Program, input string, stream bool) map[string]uint64 {
	t.Helper()
	out := map[string]uint64{}
	for _, traced := range []bool{false, true} {
		o := ParseOptions{Expectations: true, Backend: Closure}
		suffix := ""
		if traced {
			o.Trace, suffix = func(TraceEvent) {}, "+trace"
		}
		with := func(l limits.Limits) ParseOptions { x := o; x.Limits = l; return x }
		if stream {
			out["stream"+suffix] = leastCompute(t, func(l limits.Limits) error {
				return prog.ParseStreamWith("main", strings.NewReader(input), func(*value.Node) error { return nil }, with(l))
			})
			continue
		}
		out["parse"+suffix] = leastCompute(t, func(l limits.Limits) error { _, err := prog.ParseWith("main", input, with(l)); return err })
		out["recognize"+suffix] = leastCompute(t, func(l limits.Limits) error {
			x := with(l)
			x.Recognize = true
			_, err := prog.ParseWith("main", input, x)
			return err
		})
		out["prefix"+suffix] = leastCompute(t, func(l limits.Limits) error { _, err := prog.ParsePrefixWith("main", input, with(l)); return err })
		out["document"+suffix] = leastCompute(t, func(l limits.Limits) error {
			d, err := prog.NewDocumentWith("main", input, ParseOptions{Expectations: true, Backend: Closure})
			if err != nil {
				return err
			}
			_, err = d.ParseWith(with(l))
			return err
		})
	}
	return out
}

// TestClosureFamilyCharges pins the budget charge (the least compute limit) of every kind of
// closure parse, traced and untraced. The numbers are those of the closure backend when each call
// site tested Parser.NoPlain at run time: each parse runs the family that keeps them. A call of a
// plain rule through Parser.Call charges more than the plain path, so a parse that ran the wrong
// family changes a number.
func TestClosureFamilyCharges(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, src string
		opts      compile.Options
		input     string
		stream    bool
		want      map[string]uint64
	}{
		{"plain calls", "rule main = item* $$\nrule item = \"a\"", compile.Options{}, "aaaa", false, map[string]uint64{"document": 65, "document+trace": 73, "parse": 35, "parse+trace": 48, "prefix": 34, "prefix+trace": 47, "recognize": 27, "recognize+trace": 40}},
		{"plain calls, no memo", "rule main = item* $$\nrule item = \"a\"", compile.Options{DisableMemo: true}, "aaaa", false, map[string]uint64{"document": 38, "document+trace": 46, "parse": 35, "parse+trace": 48, "prefix": 34, "prefix+trace": 47, "recognize": 27, "recognize+trace": 40}},
		{"recursion", "rule main = e $$\nrule e = \"(\" e \")\" / x\nrule x = \"x\"", compile.Options{DisableMemo: true}, "((x))", false, map[string]uint64{"document": 20, "document+trace": 29, "parse": 26, "parse+trace": 39, "prefix": 25, "prefix+trace": 38, "recognize": 26, "recognize+trace": 39}},
		{"captures", "rule main = item* $$\nrule item = x:\"a\" {len(text($x)) == 1}", compile.Options{}, "aaa", false, map[string]uint64{"document": 62, "document+trace": 69, "parse": 41, "parse+trace": 48, "prefix": 40, "prefix+trace": 47, "recognize": 29, "recognize+trace": 36}},
		{"stream", gentest.Records, compile.Options{}, "#records\na=1\nbc=22\n", true, map[string]uint64{"stream": 141, "stream+trace": 151}},
	}
	for _, c := range cases {
		prog := mustCompile(t, c.src, c.opts)
		got := closureCharges(t, prog, c.input, c.stream)
		if fmt.Sprint(got) != fmt.Sprint(c.want) {
			t.Errorf("%s: charges %v, want %v", c.name, got, c.want)
		}
	}
}

// parseInFamily parses input with the start rule main of prog on the closure backend in the family
// f, with no tracer, and returns the result as ResultJSON and the parse statistics.
func parseInFamily(prog *Program, input string, unit PositionUnit, f closure.Family) (string, core.Stats) {
	p := core.AcquirePooledParser(&prog.sharedProgram)
	defer p.ReleaseBatchScratch()
	p.Input = core.NewControlledTextInput(input, unit, true, p.Scratch, nil)
	p.MaxDepth = DefaultMaxDepth
	p.Expectations = true
	p.DeferMemo = true
	p.NoPlain = f == closure.CallPreserving
	n, err := prog.Run(p, Closure, "main")
	return gentest.ResultJSON(n, err), p.ReportedStats()
}

// TestClosureFamiliesMatchOnCorpus checks that the two closure families give the same results,
// errors, expectations, recovered errors, positions and statistics on every grammar and input of
// the corpus, when no tracer observes the calls.
func TestClosureFamiliesMatchOnCorpus(t *testing.T) {
	t.Parallel()
	for _, c := range gentest.GenCorpus(t) {
		t.Run(c.Name, func(t *testing.T) {
			for _, o := range []compile.Options{{}, {DisableMemo: true}} {
				prog := mustCompile(t, c.Src, o)
				for _, in := range c.Inputs {
					for _, u := range []PositionUnit{CodePoints, Bytes} {
						a, as := parseInFamily(prog, in, u, closure.Inlined)
						b, bs := parseInFamily(prog, in, u, closure.CallPreserving)
						if a != b || as != bs {
							t.Errorf("memo off=%t input %q (%v): families differ\n inlined         %s %+v\n call-preserving %s %+v", o.DisableMemo, in, u, a, as, b, bs)
						}
					}
				}
			}
		})
	}
}

// TestClosureFamiliesBuiltOnFirstUse checks that a program builds each closure family only when a
// parse first runs it: ordinary parses the inlined one, traced and Document parses the
// call-preserving one, and that the families run distinct rules.
func TestClosureFamiliesBuiltOnFirstUse(t *testing.T) {
	t.Parallel()
	const src = "rule main = item* $$\nrule item = \"a\""
	// A parse under budget limits (the default ones too) runs the metered program that the program
	// builds for them (prepareControlledProgram), so a family counts as built in either.
	built := func(prog *Program) [2]bool {
		var got [2]bool
		for _, q := range []*Program{prog, prog.budgetProgram} {
			if q != nil {
				got[0] = got[0] || q.families[closure.Inlined].rules != nil
				got[1] = got[1] || q.families[closure.CallPreserving].rules != nil
			}
		}
		return got
	}
	prog := mustCompile(t, src)
	if got := built(prog); got != [2]bool{} {
		t.Fatalf("after Compile: built %v, want none", got)
	}
	if _, err := prog.Parse("main", "aa"); err != nil {
		t.Fatal(err)
	}
	if got := built(prog); got != [2]bool{true, false} {
		t.Fatalf("after a parse: built %v, want the inlined family", got)
	}
	if _, err := prog.ParseWith("main", "aa", ParseOptions{Trace: func(TraceEvent) {}}); err != nil {
		t.Fatal(err)
	}
	if got := built(prog); got != [2]bool{true, true} {
		t.Fatalf("after a traced parse: built %v, want both families", got)
	}
	a, err := prog.rule(Closure, "item", closure.Inlined)
	if err != nil {
		t.Fatal(err)
	}
	b, err := prog.rule(Closure, "item", closure.CallPreserving)
	if err != nil {
		t.Fatal(err)
	}
	if a == b || a.ID != b.ID || !a.Plain || !b.Plain {
		t.Fatalf("item: inlined %p (ID %d, plain %t), call-preserving %p (ID %d, plain %t)", a, a.ID, a.Plain, b, b.ID, b.Plain)
	}

	doc := mustCompile(t, src)
	d, err := doc.NewDocument("main", "aa")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Parse(); err != nil {
		t.Fatal(err)
	}
	if got := built(doc); got != [2]bool{false, true} {
		t.Fatalf("after a Document parse: built %v, want the call-preserving family", got)
	}

	stream := mustCompile(t, gentest.Records)
	if err := stream.ParseStreamWith("main", strings.NewReader("#records\na=1\n"), func(*value.Node) error { return nil }, ParseOptions{Trace: func(TraceEvent) {}}); err != nil && err != io.EOF {
		t.Fatal(err)
	}
	if got := built(stream); got != [2]bool{false, true} {
		t.Fatalf("after a traced stream parse: built %v, want the call-preserving family", got)
	}
}

// TestClosureFamiliesLeaveModuleUnchanged checks that building the closures of both families only
// reads the Core IR module that Compile stores and shares between them: the module prints the same,
// with its attributes, before and after the builds of every grammar of the corpus, and keeps no
// compile control.
func TestClosureFamiliesLeaveModuleUnchanged(t *testing.T) {
	t.Parallel()
	for _, c := range gentest.GenCorpus(t) {
		t.Run(c.Name, func(t *testing.T) {
			sp := mustCompile(t, c.Src).Static()
			m, err := closure.Core(sp, &cancel.Control{Ctx: context.Background()})
			if err != nil {
				t.Fatal(err)
			}
			if m.Control != nil {
				t.Fatal("the module keeps the compile control")
			}
			before := ir.Print(m, true)
			for _, f := range []closure.Family{closure.Inlined, closure.CallPreserving} {
				for _, plainDispatch := range []bool{true, false} {
					if _, err := closure.Build(sp, m, plainDispatch, f); err != nil {
						t.Fatal(err)
					}
					if after := ir.Print(m, true); after != before {
						t.Fatalf("family %d, plainDispatch %t: the build changed the module:\n%s\nbefore:\n%s", f, plainDispatch, after, before)
					}
				}
			}
		})
	}
}
