// Package program holds the run-time program of a grammar: the tables that a parser runs, made by
// the compiler from a checked grammar or read back from a compiled grammar file. It names no
// grammar syntax and no compiler representation: a parser reads its rules, types, variables,
// expectations and site counts here, and the code it runs (closures, bytecode and the compiled
// action expressions) comes with the rules that the compiler builds for it.
package program

import (
	"zeroparse.dev/zp/core/impl"
	"zeroparse.dev/zp/internal/bytecode"
	"zeroparse.dev/zp/internal/runtime/nodes"
)

// Program is a grammar as a parser runs it. It is immutable once built and shared by the parses
// that run it.
type Program struct {
	// RuleList holds the rules by ID; Twins the value-free twins (Rule.Twin), whose IDs follow.
	RuleList []*Rule
	Twins    []*Rule
	// ByName holds the rules of RuleList by name.
	ByName map[string]*Rule
	// DescTable is the expectation table: FixedDescs followed by the descriptions of the grammar.
	DescTable []string
	// NSeen is the number of rules with Rule.Seen set (NumberSeen).
	NSeen int
	// Vars are the declared variables (nil if there are none).
	Vars *VarTable
	// StartRules holds the start rules in declaration order and DefaultStartRule the default one.
	StartRules       []string
	DefaultStartRule string
	// HasMissing reports that the grammar uses #missing, which has no bytecode form.
	HasMissing bool
	// RecordsSourceInfo reports that nodes record source information.
	RecordsSourceInfo bool
	// MakesDefaultValue reports that value actions and declared rule types were dropped.
	MakesDefaultValue bool
	// Types holds the declared struct and node types by name (not the aliases).
	Types map[string]*Type
	// TypeKinds holds the node kinds of the grammar's types, also under the names of aliases.
	TypeKinds map[string]*coreimpl.Kind
	// NodeKinds holds the kinds of the grammar's named node types, also under the names of the
	// aliases of them.
	NodeKinds map[string]*nodes.NodeTypeKinds
	// NumSkipSites, NumRunSites and NumScanSites are the numbers of the skip-run, run and scan
	// sites, numbered in that order.
	NumSkipSites, NumRunSites, NumScanSites int
	// EnvFreeSites[site] reports that the records of the run site are not keyed by the
	// environment.
	EnvFreeSites []bool
	// RunKeys holds, by site, the key of each run site that is not free of the environment.
	RunKeys map[int]RunKey
	// Bytecode is the module of a program loaded from a compiled grammar, nil for a program that
	// the compiler built.
	Bytecode *bytecode.Module
	// InsertionCycles reports that missing-token insertion forms left-recursion cycles of its own:
	// the parses with insertion enabled run a program compiled with the rule flags of insertion.
	InsertionCycles bool
	// StructZero returns the zero descriptors of the fields of the struct type name, or nil.
	StructZero func(name string) *StructZero
}

// RunKey holds the variables that key the records of a run site that is not free of the
// environment: Vars, those that a walk from a position reads, and Bind, those that it binds and that
// a walk that takes a record sets to the values the walk of the record ended with. Both lists are
// sorted.
type RunKey struct {
	Vars, Bind []string
}

// Rule is a rule of the run-time program: what a parser needs to know to call it.
type Rule struct {
	// Where names the rule in the message of a failed action, if not Name (WhereName).
	Where string
	// Splice reports that the rule makes no node of its own: a hidden rule without an action, whose
	// elements are those of the sequence that calls it.
	Splice bool
	ID     int
	Name   string
	// Scope holds the capture slots of the rule's frame.
	Scope *Scope
	// NodeType, if not "", is the named node type the rule's value is converted to.
	NodeType string
	// OpNodeType, if not "", is the named node type every operator node of an Operators Rule is
	// converted to.
	OpNodeType string
	Memo       bool
	// Missing reports that this rule or a callee contains #missing.
	Missing bool
	Leader  bool
	// Member reports that the rule belongs to a left-recursion cycle and is not its leader.
	Member bool
	// Cycle numbers the left-recursion cycle of a leader or member from 1, and is 0 for any other
	// rule.
	Cycle int
	// LeaderIndex numbers the leaders of each cycle from 0 (NumberLeaders); LeaderBit is
	// 1<<LeaderIndex for the first MaxLeaderBits leaders of a cycle and 0 for any other rule.
	LeaderIndex int
	LeaderBit   uint64
	// Stream reports that the top level of the rule body has a #stream repetition.
	Stream bool
	// Positional reports that the result may depend on position values.
	Positional bool
	// Vars are the variables that the rule or the rules it calls read, sorted.
	Vars []string
	// Transient reports that the rule is memoized only by the incremental Document.
	Transient bool
	// Seen is the rule's index among the rules whose memoization is deferred to the second call at
	// a position, or -1 (NumberSeen).
	Seen int
	// Plain reports that the rule is never memoized except by Document and has no captures.
	Plain bool
	// Rec reports that the rule is on a cycle of the call graph: only such rules count against the
	// nesting limit.
	Rec bool
	// Novalue reports that this is a value-free rule (its value is always nil).
	Novalue bool
}

// WhereName returns the name of the rule in the message of a failed action.
func (r *Rule) WhereName() string {
	if r.Where != "" {
		return r.Where
	}
	return r.Name
}

// EntryDepth returns the nesting levels that a call of the rule adds: one for a recursive rule, none
// for any other.
func (r *Rule) EntryDepth() int {
	if r.Rec {
		return 1
	}
	return 0
}

// Defers reports whether a first call of a rule with these facts may defer its memoization
// (Rule.Seen).
func Defers(memo, transient, leader bool) bool { return memo && !transient && !leader }

// NumberSeen numbers the rules whose memoization a first call may defer (Rule.Seen) and returns
// their count.
func NumberSeen(lists ...[]*Rule) int {
	n := 0
	for _, rules := range lists {
		for _, r := range rules {
			r.Seen = -1
			if Defers(r.Memo, r.Transient, r.Leader) {
				r.Seen = n
				n++
			}
		}
	}
	return n
}

// MaxLeaderBits is the number of leaders of a cycle that have a bit (Rule.LeaderBit). The engine
// keeps the last bit of a word to tell the identity of a larger set from a word of bits.
const MaxLeaderBits = 63

// LeaderBitOf returns the bit of the leader numbered k in its cycle, 0 past MaxLeaderBits.
func LeaderBitOf(k int) uint64 {
	if k < MaxLeaderBits {
		return 1 << k
	}
	return 0
}

// NumberLeaders numbers the leaders of each cycle of rules, in order (Rule.LeaderIndex), and gives
// the first MaxLeaderBits of each a bit (Rule.LeaderBit).
func NumberLeaders(rules []*Rule) {
	n := map[int]int{}
	for _, r := range rules {
		r.LeaderIndex, r.LeaderBit = 0, 0
		if r.Leader {
			k := n[r.Cycle]
			n[r.Cycle]++
			r.LeaderIndex, r.LeaderBit = k, LeaderBitOf(k)
		}
	}
}

// Type is a declared struct or node type.
type Type struct {
	Name string
	// Fields are the names of the declared fields of a struct type, in order; nil for a node type.
	Fields []string
}

// HasField reports whether the type declares the field name.
func (t *Type) HasField(name string) bool {
	for _, f := range t.Fields {
		if f == name {
			return true
		}
	}
	return false
}

// StructZero holds the zero descriptors of the fields of a struct type: Names in declaration
// order, and the descriptor of each by name.
type StructZero struct {
	Names []string
	Zeros map[string]string
}

// TypeKind returns the node kind of the type typ.
func (p *Program) TypeKind(typ string) *coreimpl.Kind {
	if k := p.TypeKinds[typ]; k != nil {
		return k
	}
	return coreimpl.KindOf(typ, "")
}

// EnvFree reports that the records of the run site are not keyed by the environment.
func (p *Program) EnvFree(site int) bool { return site < len(p.EnvFreeSites) && p.EnvFreeSites[site] }

// StructZeroOf returns the zero descriptors of the struct type name, or nil.
func (p *Program) StructZeroOf(name string) *StructZero {
	if p.StructZero == nil {
		return nil
	}
	return p.StructZero(name)
}

// ResolveStart returns the rule name selects: name, or the default start rule when name is
// empty.
func (p *Program) ResolveStart(name string) string {
	if name == "" {
		return p.DefaultStartRule
	}
	return name
}

// CheckStart reports an error unless name selects a start rule: it is the name of a start rule,
// or empty for the default start rule.
func (p *Program) CheckStart(name string) error {
	return CheckStartName(p.StartRules, p.DefaultStartRule, p.definesRule, name)
}

// StartRule returns the start rule name selects.
func (p *Program) StartRule(name string) (*Rule, error) {
	if err := p.CheckStart(name); err != nil {
		return nil, err
	}
	return p.ByName[p.ResolveStart(name)], nil
}

// definesRule reports whether the grammar defines the rule name, without preparing a backend.
func (p *Program) definesRule(name string) bool {
	if p.Bytecode == nil {
		return p.ByName[name] != nil
	}
	m := p.Bytecode
	for _, r := range m.Rules {
		if m.Strings[r.Name] == name {
			return true
		}
	}
	return false
}
