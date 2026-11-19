package generate

import (
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"

	"zeroparse.dev/zp/internal/backend/codegen/common"
	"zeroparse.dev/zp/internal/driver"
	"zeroparse.dev/zp/internal/ir/code"
	"zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/middleend/codepass"
	"zeroparse.dev/zp/internal/middleend/corepass"
	"zeroparse.dev/zp/internal/middleend/flowpass"
	"zeroparse.dev/zp/internal/pass"
)

// Settings returns the optimizations that o.Optimize, o.Opt and o.NoOpt select, in that order, or an
// error for a profile or an optimization that does not exist, a value that it does not take, or a name
// that Opt gives twice (keys that are the same once their spaces are trimmed), and validates
// o.PrintAfter: every name it lists is a step or a pass (driver.PrintNames), and a list that is not
// empty needs o.PrintTo.
func Settings(o common.GenOptions) (pass.Settings, error) {
	_, s, err := resolve(o)
	return s, err
}

// resolve returns the names of o.PrintAfter and the settings of Settings.
func resolve(o common.GenOptions) ([]string, pass.Settings, error) {
	s, err := driver.Profile(o.Optimize)
	if err != nil {
		return nil, pass.Settings{}, err
	}
	printAfter, err := driver.ParsePrintAfter(o.PrintAfter)
	if err != nil {
		return nil, pass.Settings{}, err
	} else if len(printAfter) > 0 && o.PrintTo == nil {
		return nil, pass.Settings{}, fmt.Errorf("PrintAfter %q needs a PrintTo writer", o.PrintAfter)
	}
	given := map[string]string{}
	for _, n := range slices.Sorted(maps.Keys(o.Opt)) {
		name := strings.TrimSpace(n)
		if prev, dup := given[name]; dup {
			return nil, pass.Settings{}, fmt.Errorf("optimization %s is given twice in Opt (%q and %q)", name, prev, n)
		}
		given[name] = n
		if s, err = s.With(name, o.Opt[n]); err != nil {
			return nil, pass.Settings{}, err
		}
	}
	s, err = s.Without(o.NoOpt...)
	if err != nil {
		return nil, pass.Settings{}, err
	}
	return printAfter, s, nil
}

// Hooks are the test hooks of the pipeline of a generation (the hook fields of driver.Driver, with
// the same meaning). The settings, the dumps and the verification come from the GenOptions, so Hooks
// has no field for them.
type Hooks struct {
	CoreHook     func(pass string, m *core.Module)
	CorePipeline func(table string) *pass.Pipeline[*core.Module, *corepass.Env]
	FlowPipeline func(table string) *pass.Pipeline[*code.Module, *flowpass.Env]
	ExecPipeline func(table string) *pass.Pipeline[*code.Module, *codepass.Env]
	TestPasses   []string
}

// Passes returns the passes of a generation with o (common.Passes): the settings of Settings, and
// runners that are copies of hooks with those settings, the names of o.PrintAfter and
// o.VerifyEachPass. hooks supplies only the test hooks of the driver; the zero value runs the
// pipelines of the passes.
func Passes(o common.GenOptions, hooks Hooks) common.Passes {
	printAfter, s, err := resolve(o)
	d := driver.Driver{
		Opt: s, PrintAfter: printAfter, VerifyEach: o.VerifyEachPass,
		CoreHook: hooks.CoreHook, CorePipeline: hooks.CorePipeline, FlowPipeline: hooks.FlowPipeline,
		ExecPipeline: hooks.ExecPipeline, TestPasses: hooks.TestPasses,
	}
	return passes{driver: d, err: err}
}

// passes is the common.Passes of Passes.
type passes struct {
	driver driver.Driver // the driver of every runner, without PrintTo
	err    error         // the error of resolving the settings
}

var _ common.Runner = (*driver.Driver)(nil)

func (p passes) Settings() (pass.Settings, error) { return p.driver.Opt, p.err }

func (p passes) Runner(w io.Writer) common.Runner {
	d := p.driver
	d.PrintTo = w
	return &d
}
