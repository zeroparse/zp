package bytecode

import (
	"fmt"
	"math/bits"
	"slices"
	"sync"
)

// validateRec checks that every rule on a cycle of calls is marked Rec. Only the calls of such
// rules count against the nesting limit, so a rule that reaches a call of itself without the mark
// would recurse without bound, past the host stack. m must have passed the range checks of
// ValidateModule.
//
// The check runs on a graph that over-approximates the calls a run can make (newCallGraph). A rule
// is on a cycle of calls when its node is on a cycle of the graph: its strongly connected component
// has more than the node itself, or the node leads to itself.
func validateRec(m *Module) error {
	sc := recScratchPool.Get().(*recScratch)
	defer recScratchPool.Put(sc)
	g := newCallGraph(m, sc)
	return g.components(func(v int32, cycle bool) error {
		i := int(v) - g.rules
		if i < 0 || m.Rules[i].Rec || !cycle && !slices.Contains(g.succ(v), v) {
			return nil
		}
		return fmt.Errorf("rule %s is on a cycle of calls but is not marked recursive", m.Strings[m.Rules[i].Name])
	})
}

// recScratch holds the buffers of newCallGraph, which loads reuse.
type recScratch struct {
	begin []uint64
	buf   []int32
}

var recScratchPool = sync.Pool{New: func() any { return new(recScratch) }}

// resize returns s with length n, reusing its array when it is large enough. The elements are
// not cleared.
func resize[T any](s []T, n int) []T {
	if cap(s) < n {
		return make([]T, n)
	}
	return s[:n]
}

// callGraph is the graph of validateRec in compressed form: the successors of node v are
// succ(v). Rule i is node rules+i, and the nodes below are the blocks of the match code. work is
// scratch space for components.
type callGraph struct {
	rules  int
	start  []int32
	edges  []int32
	rindex []int32
	work   []int32
}

// succ returns the successors of node v.
func (g *callGraph) succ(v int32) []int32 { return g.edges[g.start[v]:g.start[v+1]] }

// Control flow of an operation, for target and falls: an instruction may go to its A, to its C or,
// for LOOK, to its A when it is negative (B == 1), and may not go to the next instruction.
const (
	toA = 1 << iota
	toC
	toNegativeA
	stops
)

// control holds the control flow of each operation, so that the graph reads one table entry
// for each instruction instead of switching on its operation.
var control = func() (c [256]uint8) {
	for _, op := range []Op{OpJump, OpChoice, OpCommit, OpRecover, OpEndRecover, OpIter, OpNext} {
		c[op] |= toA
	}
	c[OpLook] |= toNegativeA
	c[OpGuard] |= toC
	for _, op := range []Op{OpJump, OpCommit, OpFail, OpReturn, OpEnd} {
		c[op] |= stops
	}
	return c
}()

// target returns the instruction that in may go to other than the next one, if any: the target of
// a jump, of a choice, of a negative LOOK (a positive one has none), of a repetition's ITER and
// NEXT, of a recovery and of a GUARD.
func target(in *Instr) (int, bool) {
	switch c := control[in.Op]; {
	case c&toA != 0:
		return int(in.A), true
	case c&toNegativeA != 0:
		return int(in.A), in.B == 1
	case c&toC != 0:
		return int(in.C), true
	}
	return 0, false
}

// falls reports whether the instruction after one of operation op may run after it.
func falls(op Op) bool { return control[op]&stops == 0 }

// lines calls f with the entry of each operand and operator line of p and of its skip.
func lines(p *OperatorsRuleInfo, f func(entry int)) {
	if p.Skip >= 0 {
		f(p.Skip)
	}
	for i := range p.Operands {
		f(p.Operands[i].Entry)
	}
	for i := range p.Prefix {
		f(p.Prefix[i].Line.Entry)
	}
	for i := range p.Led {
		f(p.Led[i].Line.Entry)
	}
}

// newCallGraph returns the graph of validateRec. A block is a run of instructions that begins at
// an entry (of a rule, an Operators Rule line or skip), at a target, or after an instruction that
// does not fall through, so that control enters it only at its first instruction. A block leads to
// the blocks its instructions may go to, to the next block if its last instruction falls through,
// to the rules it calls and, for OPERATORS_RULE, to the blocks of the lines and skip. A rule leads
// to the block of its entry. Entering a block may run any of its instructions, so the graph leads
// from a rule to a rule whenever a run of the one may call the other.
//
// The graph takes two buffers of sc, whose size follows the blocks and the edges, not the
// instructions: a bit set of the instructions that begin a block, and one buffer for the
// rank of the bit set, the graph and the scratch space of components.
func newCallGraph(m *Module, sc *recScratch) callGraph {
	n := len(m.Code)
	words := (n + 63) / 64
	// begin marks the instructions that begin a block. A block's number is the count of the marks
	// before its first instruction, which rank holds for each word of begin.
	sc.begin = resize(sc.begin, words)
	begin := sc.begin
	clear(begin)
	mark := func(ip int) { begin[ip>>6] |= 1 << (ip & 63) }
	if n > 0 {
		mark(0)
	}
	for i := range m.Rules {
		mark(m.Rules[i].Entry)
	}
	for i := range m.OperatorsRules {
		lines(&m.OperatorsRules[i], mark)
	}
	// edges bounds the edges: the next block, at most one for each block, comes on top.
	edges := len(m.Rules)
	for ip := range m.Code {
		in := &m.Code[ip]
		if t, ok := target(in); ok {
			mark(t)
			edges++
		}
		if !falls(in.Op) && ip+1 < n {
			mark(ip + 1)
		}
		switch in.Op {
		case OpCall:
			edges++
		case OpOperatorsRule:
			lines(&m.OperatorsRules[in.A], func(int) { edges++ })
		}
	}
	blocks := 0
	for _, w := range begin {
		blocks += bits.OnesCount64(w)
	}
	nodes := blocks + len(m.Rules)
	edges += blocks
	sc.buf = resize(sc.buf, words+(nodes+1)+nodes+3*nodes+edges)
	buf := sc.buf
	rank, buf := buf[:words], buf[words:]
	g := callGraph{rules: blocks}
	g.start, buf = buf[:nodes+1], buf[nodes+1:]
	g.rindex, buf = buf[:nodes], buf[nodes:]
	clear(g.rindex)
	g.work, buf = buf[:3*nodes], buf[3*nodes:]
	g.edges = buf[:0]
	r := int32(0)
	for i, w := range begin {
		rank[i] = r
		r += int32(bits.OnesCount64(w))
	}
	begins := func(ip int) bool { return begin[ip>>6]&(1<<(ip&63)) != 0 }
	block := func(ip int) int32 {
		return rank[ip>>6] + int32(bits.OnesCount64(begin[ip>>6]&(1<<(ip&63)-1)))
	}
	add := func(v int32) { g.edges = append(g.edges, v) }
	addLine := func(entry int) { add(block(entry)) }
	b := 0
	for ip := range m.Code {
		if begins(ip) {
			g.start[b] = int32(len(g.edges))
			b++
		}
		in := &m.Code[ip]
		if t, ok := target(in); ok {
			add(block(t))
		}
		switch in.Op {
		case OpCall:
			add(int32(blocks) + in.A)
		case OpOperatorsRule:
			lines(&m.OperatorsRules[in.A], addLine)
		}
		if falls(in.Op) && ip+1 < n && begins(ip+1) {
			add(block(ip + 1))
		}
	}
	for i := range m.Rules {
		g.start[blocks+i] = int32(len(g.edges))
		add(block(m.Rules[i].Entry))
	}
	g.start[nodes] = int32(len(g.edges))
	return g
}

// components finds the strongly connected components of g and calls found with each node of a
// component when the component is complete, with whether it has more than one node, stopping at
// the first error. It runs Pearce's variant of Tarjan's algorithm without recursion, so that a
// long chain of blocks does not deepen the host stack: rindex holds the visit order of a node
// while it is on the stack, lowered to the least order it reaches, and a value above every order
// once its component is complete.
func (g *callGraph) components(found func(v int32, cycle bool) error) error {
	nodes := int32(len(g.rindex))
	// work holds the frames of the visits from its start, three values each (the node, its next
	// edge and its visit order), and the nodes visited and not yet in a component from its end.
	// The two sets of nodes are disjoint, so they fit.
	work := g.work
	top, low := 0, len(work)
	index, comp := int32(1), nodes // 0 is unvisited; components are numbered down from nodes
	visit := func(v int32) {
		g.rindex[v] = index
		work[top], work[top+1], work[top+2] = v, g.start[v], index
		top += 3
		index++
	}
	for root := range nodes {
		if g.rindex[root] != 0 {
			continue
		}
		visit(root)
		for top > 0 {
			f := work[top-3 : top]
			v := f[0]
			if f[1] < g.start[v+1] {
				w := g.edges[f[1]]
				f[1]++
				if g.rindex[w] == 0 {
					visit(w)
				} else {
					g.rindex[v] = min(g.rindex[v], g.rindex[w])
				}
				continue
			}
			top -= 3
			if g.rindex[v] == f[2] {
				// v is the root of its component: the nodes above it on the stack belong to it.
				index--
				cycle := low < len(work) && g.rindex[work[low]] >= g.rindex[v]
				for low < len(work) && g.rindex[work[low]] >= g.rindex[v] {
					w := work[low]
					low++
					g.rindex[w] = comp
					index--
					if err := found(w, true); err != nil {
						return err
					}
				}
				g.rindex[v] = comp
				comp--
				if err := found(v, cycle); err != nil {
					return err
				}
			} else {
				low--
				work[low] = v
			}
			if top > 0 {
				u := work[top-3]
				g.rindex[u] = min(g.rindex[u], g.rindex[v])
			}
		}
	}
	return nil
}
