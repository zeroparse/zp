package program

import "zeroparse.dev/zp/pkg/bytecode"

// VarDecl is a declared variable: its name and its initial value, an int, a string or a bool.
type VarDecl struct {
	Name string
	Init any
}

// VarTable holds the variables of a program in declaration order. A table is nil for a grammar
// without variables.
type VarTable struct {
	List  []*VarDecl
	slots map[string]int
}

// NewVarTable returns the table of the variables decls, in declaration order, or nil for none.
func NewVarTable(decls []*VarDecl) *VarTable {
	if len(decls) == 0 {
		return nil
	}
	t := &VarTable{List: decls, slots: make(map[string]int, len(decls))}
	for i, d := range decls {
		t.slots[d.Name] = i
	}
	return t
}

// All returns the variables in declaration order.
func (t *VarTable) All() []*VarDecl {
	if t == nil {
		return nil
	}
	return t.List
}

// Index returns the slot of the variable name, its position in declaration order, or -1.
func (t *VarTable) Index(name string) int {
	if t == nil {
		return -1
	}
	if i, ok := t.slots[name]; ok {
		return i
	}
	return -1
}

// Slots returns the slots of the variables names, in the same order (-1 for an undeclared name).
func (t *VarTable) Slots(names []string) []int {
	if len(names) == 0 {
		return nil
	}
	slots := make([]int, len(names))
	for i, name := range names {
		slots[i] = t.Index(name)
	}
	return slots
}

// ValuesOf returns the initial values of the variables names, or nil for no names.
func (t *VarTable) ValuesOf(names []string) []any {
	if len(names) == 0 {
		return nil
	}
	vals := make([]any, len(names))
	for i, name := range names {
		if s := t.Index(name); s >= 0 {
			vals[i] = t.List[s].Init
		}
	}
	return vals
}

// ModuleVarTable returns the variable table that the module m declares, or nil when it declares
// none. It reads back the table that the bytecode compiler writes.
func ModuleVarTable(m *bytecode.Module) *VarTable {
	var decls []*VarDecl
	for _, v := range m.Vars {
		d := &VarDecl{Name: m.Strings[v.Name]}
		switch v.Type {
		case bytecode.VarInt:
			d.Init = v.Int
		case bytecode.VarString:
			d.Init = m.Strings[v.Str]
		default:
			d.Init = v.Int == 1
		}
		decls = append(decls, d)
	}
	return NewVarTable(decls)
}
