//go:build !zp_budget_controls_reference

package main

import (
	"testing"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/engine"
)

func TestCompiledCorpusBudgetParity(t *testing.T) {
	for _, name := range []string{"json", "xml", "arithmetic", "minilang"} {
		t.Run(name, func(t *testing.T) {
			source, input, err := corpus("../..", name)
			if err != nil {
				t.Fatal(err)
			}
			p, err := engine.CompileSource(source, "main")
			if err != nil {
				t.Fatal(err)
			}
			for _, unit := range []core.PositionUnit{core.CodePoints, core.Bytes} {
				var want string
				for _, backend := range []engine.Backend{engine.Closure, engine.Bytecode, engine.BytecodeIterative} {
					for _, active := range []bool{false, true} {
						opts := []engine.ParseOption{engine.WithBackend(backend), engine.WithUnit(unit)}
						if active {
							// The largest finite budget: ^uint64(0) is Unlimited, which runs without a control.
							opts = append(opts, engine.WithLimits(core.Limits{Compute: ^uint64(0) - 1}))
						} else {
							opts = append(opts, engine.WithLimits(core.Limits{Compute: core.Unlimited}))
						}
						n, err := p.Parse(input, opts...)
						if err != nil {
							t.Fatalf("backend=%v unit=%v active=%v: %v", backend, unit, active, err)
						}
						got, err := fingerprint(n)
						if err != nil {
							t.Fatal(err)
						}
						if want == "" {
							want = got
						} else if got != want {
							t.Fatalf("backend=%v unit=%v active=%v: result differs", backend, unit, active)
						}
						n, err = p.Parse(input, append(opts, engine.RecognizeOnly())...)
						if n != nil || err != nil {
							t.Fatalf("backend=%v unit=%v active=%v: recognition node=%v error=%v", backend, unit, active, n, err)
						}
					}
				}
			}
		})
	}
}
