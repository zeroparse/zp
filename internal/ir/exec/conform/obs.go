package conform

import (
	"fmt"
	"math/rand/v2"

	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/ops"
	"zeroparse.dev/zp/internal/ir/exec/ref"
)

// ObsOps are the operations of the observation of a parse, with those their cases need to name a
// rule.
var ObsOps = []string{"obs.active", "obs.memoHit", "obs.enter", "obs.exit"}

// ObsRules is the number of rules that the cases of the observation name (ref.Config.RuleVars).
const ObsRules = 3

// ObsConfig returns the configuration of a parse of the cases of the observation: ObsRules rules, one
// variable that none reads, and an observation if observed.
func ObsConfig(observed bool) ref.Config {
	return ref.Config{Vars: []int64{0}, RuleVars: make([][]int64, ObsRules), Observed: observed}
}

// ObsModule returns the declarations of Module with those of the transitional operations, which hold
// the observation.
func ObsModule() *exec.Module {
	m := Module()
	ops.Transitional(m)
	return m
}

var obsFeatures = []string{"budget", "stream", "document", "observe"}

// ObsCases returns the handwritten conformance cases of the observation, in one module. As with Cases,
// the model checks each case first.
func ObsCases() []Case {
	m := ObsModule()
	var cs []Case
	add := func(name string, observed bool, body func(b *exec.Builder)) {
		cs = append(cs, Case{Module: m, Func: m.AddFunc(name, nil, nil, obsFeatures, body), Config: ObsConfig(observed)})
	}
	rule := func(i int64) exec.Expr { return exec.C(RuleOp, exec.Int(i)) }
	active := exec.C("obs.active")

	add("obs_unobserved", false, func(b *exec.Builder) {
		b.Assert(exec.Not(active), "a parse without observation is not observed")
		b.Do("obs.memoHit")
		b.Assert(exec.Not(active), "reporting a reuse to no observation changes nothing")
	})

	add("obs_call", true, func(b *exec.Builder) {
		b.Assert(active, "an observed parse is observed")
		b.Do("obs.enter", rule(1), exec.Int(4), exec.Int(0))
		b.Do("obs.exit", exec.BoolC(true), exec.Int(9), exec.Int(0), exec.BoolC(false))
		b.Do("obs.enter", rule(2), exec.Int(9), exec.Int(2))
		b.Do("obs.exit", exec.BoolC(false), exec.Int(9), exec.Int(12), exec.BoolC(true))
	})

	add("obs_nested", true, func(b *exec.Builder) {
		b.Do("obs.enter", rule(0), exec.Int(0), exec.Int(0))
		b.Do("obs.enter", rule(1), exec.Int(0), exec.Int(1))
		b.Do("obs.enter", rule(2), exec.Int(3), exec.Int(0))
		b.Do("obs.exit", exec.BoolC(true), exec.Int(5), exec.Int(7), exec.BoolC(true))
		b.Do("obs.exit", exec.BoolC(false), exec.Int(0), exec.Int(5), exec.BoolC(true))
		b.Do("obs.enter", rule(1), exec.Int(0), exec.Int(0))
		b.Do("obs.exit", exec.BoolC(true), exec.Int(2), exec.Int(2), exec.BoolC(false))
		b.Do("obs.exit", exec.BoolC(true), exec.Int(5), exec.Int(5), exec.BoolC(false))
	})

	add("obs_memo_hit", true, func(b *exec.Builder) {
		// A reuse belongs to the innermost call that has begun, and to no call that began after it.
		b.Do("obs.memoHit")
		b.Do("obs.enter", rule(0), exec.Int(0), exec.Int(0))
		b.Do("obs.enter", rule(1), exec.Int(2), exec.Int(0))
		b.Do("obs.memoHit")
		b.Do("obs.exit", exec.BoolC(true), exec.Int(4), exec.Int(4), exec.BoolC(false))
		b.Do("obs.enter", rule(2), exec.Int(4), exec.Int(0))
		b.Do("obs.exit", exec.BoolC(true), exec.Int(6), exec.Int(6), exec.BoolC(false))
		b.Do("obs.memoHit")
		b.Do("obs.memoHit")
		b.Do("obs.exit", exec.BoolC(false), exec.Int(0), exec.Int(6), exec.BoolC(true))
	})
	return cs
}

// RandomObs returns a randomized case of n calls of the operations of the observation, drawn with rng,
// that meets every precondition: a call ends only while one is open, and an observation exists for
// obs.enter and obs.exit only in an observed parse. Positions and levels come from small domains.
func RandomObs(name string, rng *rand.Rand, n int, observed bool) Case {
	m := ObsModule()
	cfg := ObsConfig(observed)
	mirror := ref.New(cfg)
	f := m.AddFunc(name, nil, nil, obsFeatures, func(b *exec.Builder) {
		for calls := 0; calls < n; {
			o := m.Op(ObsOps[rng.IntN(len(ObsOps))])
			var args []any
			var exprs []exec.Expr
			intArg := func(v int64) { args = append(args, v); exprs = append(exprs, exec.Int(v)) }
			boolArg := func(v bool) { args = append(args, v); exprs = append(exprs, exec.BoolC(v)) }
			switch o.Name {
			case "obs.enter":
				i := rng.Int64N(ObsRules)
				args, exprs = append(args, ref.RuleRef{Index: i}), append(exprs, exec.C(RuleOp, exec.Int(i)))
				intArg(rng.Int64N(8))
				intArg(rng.Int64N(3))
			case "obs.exit":
				boolArg(rng.IntN(2) == 0)
				intArg(rng.Int64N(8))
				intArg(rng.Int64N(10))
				boolArg(rng.IntN(2) == 0)
			}
			if !mirror.Allows(o, args) {
				continue
			}
			mirror.Call(o, args)
			calls++
			if len(o.Results) > 0 {
				b.Let(fmt.Sprintf("x%d", calls), exec.C(o.Name, exprs...))
			} else {
				b.Do(o.Name, exprs...)
			}
		}
	})
	return Case{Module: m, Func: f, Config: cfg}
}
