package engine

import (
	"zeroparse.dev/zp/internal/engine/state"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/internal/compile"
)

// PrefixResult records the successful cursor independently of the rule's value.
type PrefixResult struct {
	Node          *core.Node
	Consumed      int
	ConsumedBytes int
}

// ParsePrefixWith parses a named rule without an implicit whole-input check.
// Recovered success returns consumption and SyntaxErrors; unrecovered failures
// and fatal errors return a zero result. Explicit EOF assertions still see the
// entire supplied input.
func (prog *Program) ParsePrefixWith(start, input string, o ParseOptions) (result PrefixResult, err error) {
	if err := state.ValidateStreamDiagnostics(o, false); err != nil {
		return PrefixResult{}, err
	}
	if prog.HasMissing || o.MissingTokens > 0 {
		return PrefixResult{}, compile.MissingUnsupported("prefix parsing")
	}
	defer func() {
		if x := recover(); x != nil {
			if f, ok := x.(state.Fatal); ok {
				result, err = PrefixResult{}, f.Err
			} else if cp, ok := x.(state.ContextPanic); ok {
				panic(cp.Value)
			} else {
				panic(x)
			}
		}
	}()
	start = prog.ResolveStart(start) // an empty name selects the default start rule
	if err := prog.checkMode(o, false); err != nil {
		return PrefixResult{}, err
	}
	active, _, control, err := prog.admitParse(start, o, optionsFamily(o), false, input)
	if err != nil {
		return PrefixResult{}, err
	}
	control.Check()
	p := state.AcquirePooledParser(&active.sharedProgram)
	defer p.ReleaseBatchScratch()
	p.Input = state.NewControlledTextInput(input, o.Unit, !o.Recognize, p.Scratch, control)
	control.AttachMemory(p)
	p.MaxDepth, _ = state.DepthLimit(o, o.Backend) // admitParse rejected an invalid limit
	p.Expectations = o.Expectations
	p.DeferMemo = true
	p.SetTrace(o.Trace)
	result, err = active.runPrefix(p, o.Backend, start)
	if !p.Aborted {
		control.Check()
	}
	if o.Recognize {
		result.Node = nil
	}
	result.ConsumedBytes = p.Input.PrefixByteOffset(input, result.Consumed)

	if !p.Aborted {
		control.Check()
	}
	return result, err
}

func (prog *Program) runPrefix(p *state.Parser, b Backend, start string) (result PrefixResult, err error) {
	p.Aborted = false
	r, err := prog.rule(b, start, state.FamilyOf(p), p.Control)
	if err != nil {
		return PrefixResult{}, err
	}
	p.Descs = prog.descs(b, state.FamilyOf(p))
	p.Memo.Stride = prog.NSeen
	if be := prog.backend(b); be == Bytecode || be == BytecodeIterative {
		p.Memo.Stride = prog.vmFor(be, state.FamilyOf(p)).NumSeen()
	}
	p.Memo.Sparse = state.SparseMemo && p.Memo.Stride > 64 && p.Loaded() > 1024
	defer func() {
		if x := recover(); x != nil {
			p.Aborted = true
			result = PrefixResult{}
			recoverParse(x, prog.backend(b), &err)
		}
	}()
	p.SeedVars()
	v, ok := p.Call(r, 0)
	if !ok {
		return PrefixResult{}, p.SyntaxError()
	}
	result = PrefixResult{Node: v, Consumed: p.Pos}
	if len(p.Recovered) > 0 {
		err = core.SyntaxErrors(p.RecoveredErrors())
	}
	return result, err
}
