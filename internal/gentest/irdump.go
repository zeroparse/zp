package gentest

import (
	"slices"

	"zeroparse.dev/zp/internal/ir/flow"
	"zeroparse.dev/zp/internal/ir/pipeline"
)

// BrokenFlowPass is a Flow pass that lists a variable that is also an entry variable, which flow.Verify
// rejects.
var BrokenFlowPass = pipeline.Pass[*flow.Func, *flow.Env]{
	Name:       "break-flow",
	Obligation: "none: it breaks the function on purpose",
	Run: func(f *flow.Func, _ *flow.Env) error {
		f.Vars = append(f.Vars, &flow.Var{Name: "broken", Type: flow.TypeInt, Entry: true})
		return nil
	},
}

// BrokenFlowRun returns a FlowRun hook of a generator that runs the Flow pipeline with
// BrokenFlowPass added at its end on the functions of the table named table, and the Flow pipeline
// as it is on the others. It counts the functions of the table in seen.
func BrokenFlowRun(table string, seen *int) func(string, *flow.Func, *flow.Env, flow.RunOptions) ([]string, error) {
	p := *flow.Pipeline
	p.Passes = append(slices.Clone(flow.Pipeline.Passes), BrokenFlowPass)
	return func(tb string, f *flow.Func, env *flow.Env, o flow.RunOptions) ([]string, error) {
		if tb != table {
			return flow.Run(f, env, o)
		}
		*seen++
		po := o.Options(env)
		// The pass is not a registered optimization, which the settings of the generator would refuse.
		po.Enabled = func(name string) bool { return name == BrokenFlowPass.Name || env.On(name) }
		return p.Run(f, env, po)
	}
}
