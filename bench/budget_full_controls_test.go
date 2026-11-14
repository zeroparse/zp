//go:build !zp_budget_controls_reference

package bench

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/core/impl"
	"zeroparse.dev/zp/engine"
	gcalc "zeroparse.dev/zp/internal/bench/parsers/calc"
	gcalclr "zeroparse.dev/zp/internal/bench/parsers/calclr"
	gcsv "zeroparse.dev/zp/internal/bench/parsers/csv"
	gjson "zeroparse.dev/zp/internal/bench/parsers/json"
	gminilang "zeroparse.dev/zp/internal/bench/parsers/minilang"
	goutline "zeroparse.dev/zp/internal/bench/parsers/outline"
	gxml "zeroparse.dev/zp/internal/bench/parsers/xml"
	"zeroparse.dev/zp/internal/value"
)

type fullGeneratedProbe struct {
	parse, ast func(string, bool, bool) (any, error)
	recognize  func(string, bool, bool) error
}

func fullProbe[N, A, O any](parse func(string, ...O) (N, error), ast func(string, ...O) (A, error), rec func(string, ...O) error, opts func(bool, bool) []O) fullGeneratedProbe {
	return fullGeneratedProbe{func(s string, b, c bool) (any, error) { return parse(s, opts(b, c)...) }, func(s string, b, c bool) (any, error) { return ast(s, opts(b, c)...) }, func(s string, b, c bool) error { return rec(s, opts(b, c)...) }}
}

func fullGeneratedProbes() map[string]fullGeneratedProbe {
	return map[string]fullGeneratedProbe{
		"JSON": fullProbe(gjson.Parse, gjson.ParseValue, gjson.Recognize, func(b, c bool) []gjson.ParseOption {
			u := gjson.CodePoints
			if b {
				u = gjson.Bytes
			}
			o := []gjson.ParseOption{gjson.WithUnit(u)}
			if c {
				// The largest finite budget: ^uint64(0) is Unlimited, which runs without a control.
				o = append(o, gjson.WithLimits(gjson.Limits{Compute: ^uint64(0) - 1}))
			} else {
				// A zero budget selects the default counter; the uncontrolled form needs Unlimited.
				o = append(o, gjson.WithLimits(gjson.Limits{Compute: gjson.Unlimited}))
			}
			return o
		}),
		"CSV": fullProbe(gcsv.Parse, gcsv.ParseValue, gcsv.Recognize, func(b, c bool) []gcsv.ParseOption {
			u := gcsv.CodePoints
			if b {
				u = gcsv.Bytes
			}
			o := []gcsv.ParseOption{gcsv.WithUnit(u)}
			if c {
				o = append(o, gcsv.WithLimits(gcsv.Limits{Compute: ^uint64(0) - 1}))
			} else {
				// A zero budget selects the default counter; the uncontrolled form needs Unlimited.
				o = append(o, gcsv.WithLimits(gcsv.Limits{Compute: gcsv.Unlimited}))
			}
			return o
		}),
		"XML": fullProbe(gxml.Parse, gxml.ParseValue, gxml.Recognize, func(b, c bool) []gxml.ParseOption {
			u := gxml.CodePoints
			if b {
				u = gxml.Bytes
			}
			o := []gxml.ParseOption{gxml.WithUnit(u)}
			if c {
				o = append(o, gxml.WithLimits(gxml.Limits{Compute: ^uint64(0) - 1}))
			} else {
				// A zero budget selects the default counter; the uncontrolled form needs Unlimited.
				o = append(o, gxml.WithLimits(gxml.Limits{Compute: gxml.Unlimited}))
			}
			return o
		}),
		"Arith_OperatorsRule": fullProbe(gcalc.Parse, gcalc.ParseValue, gcalc.Recognize, func(b, c bool) []gcalc.ParseOption {
			u := gcalc.CodePoints
			if b {
				u = gcalc.Bytes
			}
			o := []gcalc.ParseOption{gcalc.WithUnit(u)}
			if c {
				o = append(o, gcalc.WithLimits(gcalc.Limits{Compute: ^uint64(0) - 1}))
			} else {
				// A zero budget selects the default counter; the uncontrolled form needs Unlimited.
				o = append(o, gcalc.WithLimits(gcalc.Limits{Compute: gcalc.Unlimited}))
			}
			return o
		}),
		"Arith_LeftRec": fullProbe(gcalclr.Parse, gcalclr.ParseValue, gcalclr.Recognize, func(b, c bool) []gcalclr.ParseOption {
			u := gcalclr.CodePoints
			if b {
				u = gcalclr.Bytes
			}
			o := []gcalclr.ParseOption{gcalclr.WithUnit(u)}
			if c {
				o = append(o, gcalclr.WithLimits(gcalclr.Limits{Compute: ^uint64(0) - 1}))
			} else {
				// A zero budget selects the default counter; the uncontrolled form needs Unlimited.
				o = append(o, gcalclr.WithLimits(gcalclr.Limits{Compute: gcalclr.Unlimited}))
			}
			return o
		}),
		"Minilang": fullProbe(gminilang.Parse, gminilang.ParseValue, gminilang.Recognize, func(b, c bool) []gminilang.ParseOption {
			u := gminilang.CodePoints
			if b {
				u = gminilang.Bytes
			}
			o := []gminilang.ParseOption{gminilang.WithUnit(u)}
			if c {
				o = append(o, gminilang.WithLimits(gminilang.Limits{Compute: ^uint64(0) - 1}))
			} else {
				// A zero budget selects the default counter; the uncontrolled form needs Unlimited.
				o = append(o, gminilang.WithLimits(gminilang.Limits{Compute: gminilang.Unlimited}))
			}
			return o
		}),
		"Recovery": fullProbe(gminilang.Parse, gminilang.ParseValue, gminilang.Recognize, func(b, c bool) []gminilang.ParseOption {
			u := gminilang.CodePoints
			if b {
				u = gminilang.Bytes
			}
			o := []gminilang.ParseOption{gminilang.WithUnit(u)}
			if c {
				o = append(o, gminilang.WithLimits(gminilang.Limits{Compute: ^uint64(0) - 1}))
			} else {
				// A zero budget selects the default counter; the uncontrolled form needs Unlimited.
				o = append(o, gminilang.WithLimits(gminilang.Limits{Compute: gminilang.Unlimited}))
			}
			return o
		}),
		"Outline": fullProbe(goutline.Parse, goutline.ParseValue, goutline.Recognize, func(b, c bool) []goutline.ParseOption {
			u := goutline.CodePoints
			if b {
				u = goutline.Bytes
			}
			o := []goutline.ParseOption{goutline.WithUnit(u)}
			if c {
				o = append(o, goutline.WithLimits(goutline.Limits{Compute: ^uint64(0) - 1}))
			} else {
				// A zero budget selects the default counter; the uncontrolled form needs Unlimited.
				o = append(o, goutline.WithLimits(goutline.Limits{Compute: goutline.Unlimited}))
			}
			return o
		}),
	}
}

func fullDiagnostics(t *testing.T, err error, recovered bool) string {
	t.Helper()
	if !recovered && err != nil {
		t.Fatalf("unexpected diagnostics: %v", err)
	}
	if recovered {
		if err == nil || reflect.TypeOf(err).Name() != "SyntaxErrors" || reflect.ValueOf(err).Len() == 0 {
			t.Fatalf("expected recovered SyntaxErrors: %T %v", err, err)
		}
	}
	data, e := json.Marshal(err)
	if e != nil {
		t.Fatal(e)
	}
	if err == nil {
		return string(data)
	}
	// Keep raw diagnostic bytes too; JSON replaces invalid UTF-8.
	return reflect.TypeOf(err).String() + string(data) + err.Error()
}

// Full-size fingerprints cover positions, all fields/children, scalar values,
// sharing and nil distinctions, not only successful parsing on reduced input.
func TestControlledFullWorkloads(t *testing.T) {
	probes := fullGeneratedProbes()
	for _, w := range workloads {
		t.Run(w.name, func(t *testing.T) {
			t.Parallel()
			input := w.input(1)
			p := load(t, w.grammar)
			check := func(n any, err error) string {
				t.Helper()
				if n == nil || reflect.ValueOf(n).Kind() == reflect.Ptr && reflect.ValueOf(n).IsNil() {
					t.Fatal("full workload did not return a tree")
				}
				if err != nil && !w.partial {
					t.Fatal(err)
				}
				hash, e := fullFingerprint(n)
				if e != nil {
					t.Fatal(e)
				}
				return hash + fullDiagnostics(t, err, w.partial)
			}
			for _, be := range backends {
				for _, u := range units {
					t.Run(be.name+"/"+u.name, func(t *testing.T) {
						t.Parallel()
						// Unlimited keeps the plain parse uncontrolled: a zero budget selects the default counter.
						opts := []engine.ParseOption{engine.WithBackend(be.b), engine.WithUnit(u.u), engine.WithLimits(core.Limits{Compute: core.Unlimited})}
						plain, plainErr := p.Parse(input, opts...)
						want := check(plain, plainErr)
						for _, mode := range []string{"budget", "context", "combined"} {
							active := append([]engine.ParseOption{}, opts...)
							if mode != "context" {
								active = append(active, engine.WithLimits(core.Limits{Compute: ^uint64(0) - 1}))
							}
							if mode != "budget" {
								ctx, cancel := context.WithCancel(context.Background())
								defer cancel()
								active = append(active, engine.WithContext(ctx))
							}
							n, err := p.Parse(input, active...)
							if got := check(n, err); got != want {
								t.Fatalf("%s full fingerprint differs", mode)
							}
							recNode, recErr := p.Parse(input, append(active, engine.RecognizeOnly())...)
							plainRecNode, plainRecErr := p.Parse(input, append(opts, engine.RecognizeOnly())...)
							if recNode != nil || plainRecNode != nil {
								t.Fatal("recognition returned a tree")
							}
							a := fullDiagnostics(t, recErr, w.partial)
							b := fullDiagnostics(t, plainRecErr, w.partial)
							if a != b {
								t.Fatalf("%s recognition diagnostics differ", mode)
							}
						}
						n, err := p.Parse(input, append(opts, engine.WithLimits(core.Limits{Compute: 1}))...)
						if n != nil || !errors.Is(err, core.ErrLimit) {
							t.Fatalf("full-input interruption: %v", err)
						}
						retry, retryErr := p.Parse(input, opts...)
						if check(retry, retryErr) != want {
							t.Fatal("retry changed full result")
						}
					})
				}
			}
			g := probes[w.name]
			for _, bytes := range []bool{false, true} {
				a, ae := g.parse(input, bytes, false)
				b, be := g.parse(input, bytes, true)
				if check(a, ae) != check(b, be) {
					t.Fatal("generated Node full result differs")
				}
				av, aerr := g.ast(input, bytes, false)
				bv, berr := g.ast(input, bytes, true)
				for _, v := range []any{av, bv} {
					if v == nil || reflect.ValueOf(v).Kind() == reflect.Ptr && reflect.ValueOf(v).IsNil() {
						t.Fatal("full typed workload did not return a value")
					}
				}
				if aerr != nil && !w.partial || berr != nil && !w.partial {
					t.Fatalf("typed errors: %v / %v", aerr, berr)
				}
				aj, e := json.Marshal(av)
				if e != nil {
					t.Fatal(e)
				}
				bj, e := json.Marshal(bv)
				if e != nil {
					t.Fatal(e)
				}
				ax := fullDiagnostics(t, aerr, w.partial)
				bx := fullDiagnostics(t, berr, w.partial)
				if string(aj) != string(bj) || ax != bx {
					t.Fatal("generated typed full output/diagnostics differ")
				}
				ar := g.recognize(input, bytes, false)
				br := g.recognize(input, bytes, true)
				arj := fullDiagnostics(t, ar, w.partial)
				brj := fullDiagnostics(t, br, w.partial)
				if arj != brj {
					t.Fatal("generated recognition diagnostics differ")
				}
			}
		})
	}
}

func TestFullFingerprintDetectsObservableGraphChanges(t *testing.T) {
	hash := func(n *core.Node) string {
		t.Helper()
		s, e := fullFingerprint(n)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	str := func(start, end int, text string) *core.Node {
		return value.NewNode(core.KindString, start, end, text, nil, "", nil, nil)
	}
	tuple := func(elems ...*core.Node) *core.Node {
		return value.NewNode(core.KindTuple, 0, 0, "", elems, "", nil, nil)
	}
	object := func(fields ...coreimpl.NodeField) *core.Node {
		return value.NewNode(core.KindStruct, 0, 0, "", nil, "T", coreimpl.Fields(fields), nil)
	}
	if hash(str(0, 0, "\xff")) == hash(str(0, 0, "\xfe")) {
		t.Fatal("raw invalid terminal bytes were replaced")
	}
	if hash(object(coreimpl.NodeField{Name: "x", Value: "\xff"})) == hash(object(coreimpl.NodeField{Name: "x", Value: "\xfe"})) {
		t.Fatal("raw invalid scalar bytes were replaced")
	}
	leaf := str(0, 0, "x")
	shared := tuple(leaf, leaf)
	copied := tuple(str(0, 0, "x"), str(0, 0, "x"))
	if hash(shared) == hash(copied) {
		t.Fatal("sharing was discarded")
	}
	fields := object(coreimpl.NodeField{Name: "a", Value: "one"}, coreimpl.NodeField{Name: "b", Value: "two"})
	reordered := object(coreimpl.NodeField{Name: "b", Value: "two"}, coreimpl.NodeField{Name: "a", Value: "one"})
	if hash(fields) == hash(reordered) {
		t.Fatal("field order was discarded")
	}
	if hash(tuple()) == hash(value.NewNode(core.KindList, 0, 0, "", nil, "", nil, nil)) {
		t.Fatal("the kind of an empty node was discarded")
	}
	shifted := str(1, 2, "x")
	if hash(leaf) == hash(shifted) {
		t.Fatal("positions were discarded")
	}
	changed := object(coreimpl.NodeField{Name: "a", Value: "changed"}, coreimpl.NodeField{Name: "b", Value: "two"})
	if hash(fields) == hash(changed) {
		t.Fatal("field values were discarded")
	}
	if hash(value.NewNode(core.KindValue, 0, 0, "", nil, "", nil, 1)) == hash(value.NewNode(core.KindValue, 0, 0, "", nil, "", nil, 2)) {
		t.Fatal("carried values were discarded")
	}
	cycle := tuple()
	value.SetNodeElems(cycle, []*core.Node{cycle})
	other := tuple()
	value.SetNodeElems(other, []*core.Node{other})
	if hash(cycle) != hash(other) {
		t.Fatal("equivalent cycles differ")
	}
}
