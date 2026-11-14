//go:build !zp_budget_controls_reference

package engine

import (
	"fmt"
	"io"
	"strings"
	"testing"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/internal/codegentest"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/engine/state"
)

var vmBackends = []Backend{Bytecode, BytecodeIterative}

// TestVMFamilyCharges pins the budget charge (the least compute limit) of every kind of parse on
// both VMs, traced and untraced. The numbers are those of the VMs when each call site tested
// Parser.NoPlain at run time: each parse runs the family that keeps them. A call of a plain rule
// through Parser.Call charges more than the plain path, so a parse that ran the wrong family
// changes a number.
func TestVMFamilyCharges(t *testing.T) {
	t.Parallel()
	type charges = map[string]uint64
	// The stream parses of the default build also charge for walking and clearing the capture
	// frames they keep for reuse at each commit, which the reference build of stream frames drops.
	stream := map[Backend]charges{Bytecode: {"stream": 152, "stream+trace": 162}, BytecodeIterative: {"stream": 152, "stream+trace": 157}}
	if !state.StreamFrameScratch {
		stream = map[Backend]charges{Bytecode: {"stream": 148, "stream+trace": 158}, BytecodeIterative: {"stream": 148, "stream+trace": 153}}
	}
	cases := []struct {
		name, src string
		opts      compile.Options
		input     string
		stream    bool
		want      map[Backend]charges
	}{
		{"plain calls", "rule main = item* $$\nrule item = \"a\"", compile.Options{}, "aaaa", false, map[Backend]charges{
			Bytecode:          {"document": 67, "document+trace": 75, "parse": 33, "parse+trace": 46, "prefix": 32, "prefix+trace": 45, "recognize": 28, "recognize+trace": 41},
			BytecodeIterative: {"document": 67, "document+trace": 70, "parse": 33, "parse+trace": 36, "prefix": 32, "prefix+trace": 35, "recognize": 28, "recognize+trace": 31},
		}},
		{"plain calls, no memo", "rule main = item* $$\nrule item = \"a\"", compile.Options{DisableMemo: true}, "aaaa", false, map[Backend]charges{
			Bytecode:          {"document": 40, "document+trace": 48, "parse": 33, "parse+trace": 46, "prefix": 32, "prefix+trace": 45, "recognize": 28, "recognize+trace": 41},
			BytecodeIterative: {"document": 35, "document+trace": 38, "parse": 33, "parse+trace": 36, "prefix": 32, "prefix+trace": 35, "recognize": 28, "recognize+trace": 31},
		}},
		{"recursion", "rule main = e $$\nrule e = \"(\" e \")\" / x\nrule x = \"x\"", compile.Options{DisableMemo: true}, "((x))", false, map[Backend]charges{
			Bytecode:          {"document": 27, "document+trace": 36, "parse": 33, "parse+trace": 46, "prefix": 32, "prefix+trace": 45, "recognize": 26, "recognize+trace": 39},
			BytecodeIterative: {"document": 23, "document+trace": 28, "parse": 33, "parse+trace": 38, "prefix": 32, "prefix+trace": 37, "recognize": 26, "recognize+trace": 31},
		}},
		{"captures", "rule main = item* $$\nrule item = x:\"a\" {len(text($x)) == 1}", compile.Options{}, "aaa", false, map[Backend]charges{
			Bytecode:          {"document": 67, "document+trace": 74, "parse": 43, "parse+trace": 50, "prefix": 42, "prefix+trace": 49, "recognize": 30, "recognize+trace": 37},
			BytecodeIterative: {"document": 67, "document+trace": 70, "parse": 43, "parse+trace": 46, "prefix": 42, "prefix+trace": 45, "recognize": 30, "recognize+trace": 33},
		}},
		{"stream", codegentest.Records, compile.Options{}, "#records\na=1\nbc=22\n", true, stream},
	}
	for _, c := range cases {
		for _, b := range vmBackends {
			prog := mustCompile(t, c.src, c.opts)
			got := familyCharges(t, prog, b, c.input, c.stream)
			if fmt.Sprint(got) != fmt.Sprint(c.want[b]) {
				t.Errorf("%s on %s: charges %v, want %v", c.name, b, got, c.want[b])
			}
		}
	}
}

// TestVMFamiliesMatchOnCorpus checks that the two families of each VM give the same results,
// errors, expectations, recovered errors, positions and statistics on every grammar and input of
// the corpus, when no tracer observes the calls.
func TestVMFamiliesMatchOnCorpus(t *testing.T) {
	t.Parallel()
	for _, c := range codegentest.GenCorpus(t) {
		t.Run(c.Name, func(t *testing.T) {
			for _, o := range []compile.Options{{}, {DisableMemo: true}} {
				prog := mustCompile(t, c.Src, o)
				if prog.HasMissing {
					continue
				}
				for _, b := range vmBackends {
					for _, in := range c.Inputs {
						for _, u := range []core.PositionUnit{core.CodePoints, core.Bytes} {
							x, xs := parseInFamily(prog, b, in, u, state.Inlined)
							y, ys := parseInFamily(prog, b, in, u, state.CallPreserving)
							if x != y || xs != ys {
								t.Errorf("%s memo off=%t input %q (%v): families differ\n inlined         %s %+v\n call-preserving %s %+v", b, o.DisableMemo, in, u, x, xs, y, ys)
							}
						}
					}
				}
			}
		})
	}
}

// TestVMFamiliesPreparedOnFirstUse checks that a program prepares the VM program of each family
// only when a parse first runs it: ordinary parses the inlined one, traced and Document parses the
// call-preserving one, and that the families run distinct rules (TestVMFamilyCharges checks which
// family calls plain rules directly).
func TestVMFamiliesPreparedOnFirstUse(t *testing.T) {
	t.Parallel()
	const src = "rule main = item* $$\nrule item = \"a\""
	for _, b := range vmBackends {
		prepared := func(prog *Program) [2]bool {
			v := &prog.vms[vmIndex(b)]
			return [2]bool{v[state.Inlined].vm != nil, v[state.CallPreserving].vm != nil}
		}
		o := ParseOptions{Backend: b}
		prog := mustCompile(t, src)
		if _, err := prog.ParseWith("main", "aa", o); err != nil {
			t.Fatal(err)
		}
		if got := prepared(prog); got != [2]bool{true, false} {
			t.Fatalf("%s: after a parse: prepared %v, want the inlined family", b, got)
		}
		traced := o
		traced.Trace = func(core.TraceEvent) {}
		if _, err := prog.ParseWith("main", "aa", traced); err != nil {
			t.Fatal(err)
		}
		if got := prepared(prog); got != [2]bool{true, true} {
			t.Fatalf("%s: after a traced parse: prepared %v, want both families", b, got)
		}
		in, err := prog.rule(b, "item", state.Inlined, nil)
		if err != nil {
			t.Fatal(err)
		}
		cp, err := prog.rule(b, "item", state.CallPreserving, nil)
		if err != nil {
			t.Fatal(err)
		}
		if in == cp || in.ID != cp.ID || !in.Plain || !cp.Plain {
			t.Fatalf("%s: item: inlined %p (ID %d, plain %t), call-preserving %p (ID %d, plain %t)", b, in, in.ID, in.Plain, cp, cp.ID, cp.Plain)
		}

		doc := mustCompile(t, src)
		d, err := doc.NewDocumentWith("main", "aa", o)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := d.Parse(); err != nil {
			t.Fatal(err)
		}
		if got := prepared(doc); got != [2]bool{false, true} {
			t.Fatalf("%s: after a Document parse: prepared %v, want the call-preserving family", b, got)
		}

		stream := mustCompile(t, codegentest.Records)
		if err := stream.ParseStreamWith("main", strings.NewReader("#records\na=1\n"), func(*core.Node) error { return nil }, traced); err != nil && err != io.EOF {
			t.Fatal(err)
		}
		if got := prepared(stream); got != [2]bool{false, true} {
			t.Fatalf("%s: after a traced stream parse: prepared %v, want the call-preserving family", b, got)
		}
	}
}
