package engine

import (
	"fmt"
	"strings"
	"sync"

	"zeroparse.dev/zp/internal/backend/codegen/golang/print"
	"zeroparse.dev/zp/internal/ir/code"
	"zeroparse.dev/zp/internal/ir/code/flow"
	"zeroparse.dev/zp/internal/ir/core"
	"zeroparse.dev/zp/internal/middleend/flowpass"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

// The terminal programs, the class runs and the guarded classes are chosen by the Flow passes of the
// target (internal/middleend/flowpass), not by the Core passes. The decisions golden freezes them with
// the decisions of Core IR: the generations of the tests record what the Flow passes replaced in each
// function the generator lowered, and coreDecisionLines encodes the records of the case's table.

// flowKey is the place of a decision: a rule of the table and an expression of its body.
type flowKey struct {
	rule int
	e    ast.Expr
}

// flowDecisions are the decisions of the Flow passes on the functions lowered from one table, in
// the copies of one kind (traced or not).
type flowDecisions struct {
	// terminals are the terminal regions matched as one program, by their root's expression.
	terminals map[flowKey]bool
	// runs are the class runs of each value-free sequence, as "position:length" in its items.
	runs map[flowKey]string
	// guarded are the alternatives, by their expression (coreGenCase.altOrigin), whose first class
	// consumes the character of the guard.
	guarded map[flowKey]bool
}

func newFlowDecisions() *flowDecisions {
	return &flowDecisions{terminals: map[flowKey]bool{}, runs: map[flowKey]string{}, guarded: map[flowKey]bool{}}
}

// clone returns a copy of d that a negative control may change.
func (d *flowDecisions) clone() *flowDecisions {
	c := newFlowDecisions()
	for k, v := range d.terminals {
		c.terminals[k] = v
	}
	for k, v := range d.runs {
		c.runs[k] = v
	}
	for k, v := range d.guarded {
		c.guarded[k] = v
	}
	return c
}

// flowScope says which functions a record holds: those of a rule table, traced or not.
type flowScope struct {
	table  string
	traced bool
}

// flowRecord holds the decisions of the Flow passes in one generation.
type flowRecord struct {
	mu  sync.Mutex
	gen *print.Generator
	by  map[flowScope]*flowDecisions
}

// decisions returns the decisions recorded for the table, in the copies a case of gen reads: the
// traced ones in a build with observability (coreGenerate), the others otherwise.
func (r *flowRecord) decisions(table string, gen *print.Generator) *flowDecisions {
	if r == nil {
		return newFlowDecisions()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if d, ok := r.by[flowScope{table, gen.Traced}]; ok {
		return d
	}
	return newFlowDecisions()
}

// flowRecords maps each generator of coreGenerate to the record of its generation.
var flowRecords sync.Map // *print.Generator -> *flowRecord

// flowRecordOf returns the record of the generation of gen, or nil.
func flowRecordOf(gen *print.Generator) *flowRecord {
	r, _ := flowRecords.Load(gen)
	rec, _ := r.(*flowRecord)
	return rec
}

// recordingHooks returns hooks that keep the last generator that prints in rec, as the generator of
// the case, and record in rec, after the Flow passes of each function that a generator lowers from
// Core IR, what the passes replaced. A generation may print with several generators, so each records
// with the Core IR of its own table.
func recordingHooks(rec *flowRecord) *print.Hooks {
	return &print.Hooks{
		Generator: func(x *print.Generator) {
			rec.gen = x
			driverOf(x).FlowRun = func(_ string, m *code.Module, env *flowpass.Env, o flowpass.RunOptions) ([]string, error) {
				ran, err := flowpass.Run(m, env, o)
				if err == nil {
					rec.record(x, m)
				}
				return ran, err
			}
		},
	}
}

// record adds the decisions of the passes on the function of m, which the generator g lowered from
// the Core IR of its current table (codegen.FlowState), before flowpass.Finish drops its forms.
func (rec *flowRecord) record(g *print.Generator, m *code.Module) {
	a := flow.AttrsOf(flow.FuncOf(m))
	if a == nil || a.Forms == nil || a.Rule == nil || g.Flow == nil {
		return
	}
	origin := g.Flow.Origin
	rec.mu.Lock()
	defer rec.mu.Unlock()
	if rec.by == nil {
		rec.by = map[flowScope]*flowDecisions{}
	}
	scope := flowScope{g.Table, a.Traced}
	d := rec.by[scope]
	if d == nil {
		d = newFlowDecisions()
		rec.by[scope] = d
	}
	id := a.Rule.ID
	for _, r := range a.Forms.Regions {
		if r.By != flowpass.PassTerminals {
			continue
		}
		if e, ok := origin[r.Node]; ok {
			d.terminals[flowKey{id, e}] = true
		}
	}
	for _, s := range a.Forms.Seqs {
		e, ok := origin[s.Node]
		if !ok {
			continue
		}
		var runs []string
		for i := 0; i < len(s.Items); i++ {
			n := 0
			for i+n < len(s.Items) && s.Items[i+n] != nil && s.Items[i+n].By == flowpass.PassClassRuns {
				n++
			}
			if n > 0 {
				runs = append(runs, fmt.Sprintf("%d:%d", i, n))
				i += n - 1
			}
		}
		if len(runs) > 0 {
			d.runs[flowKey{id, e}] = strings.Join(runs, " ")
		}
	}
	for _, cl := range a.Forms.Classes {
		if cl.By != flowpass.PassGuardedClasses || cl.Guard == nil {
			continue
		}
		x := cl.Guard.Node
		for i, alt := range x.Alts {
			if leadingClass(alt.Body) != cl.Node {
				continue
			}
			if e, ok := altOriginIn(origin, x, i); ok {
				d.guarded[flowKey{id, e}] = true
			}
		}
	}
}

// leadingClass returns the class that n is or that begins the sequence n, or nil.
func leadingClass(n core.Node) core.Node {
	if s, ok := n.(*core.Seq); ok && len(s.Items) > 0 {
		n = s.Items[0]
	}
	if c, ok := n.(*core.ClassNode); ok {
		return c
	}
	return nil
}

// altOriginIn returns the expression of the alternative i of the choice x under origin, as
// coreGenCase.altOrigin does.
func altOriginIn(origin map[core.Node]ast.Expr, x *core.Choice, i int) (ast.Expr, bool) {
	if ce, ok := origin[x].(*ast.Choice); ok && len(ce.Alts) == len(x.Alts) {
		return ce.Alts[i], true
	}
	e, ok := origin[x.Alts[i].Body]
	return e, ok
}
