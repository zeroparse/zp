// Package check holds the checker of the value model: the variable table, type resolution, value
// inference and term checking, over the type model in internal/types.
package check

import (
	"zeroparse.dev/zp/internal/bytecode"
	"zeroparse.dev/zp/internal/types"
)

// A parser variable is declared by a top-level var declaration. Its type is int, string or
// bool, and it starts every parse with its initial value, which is the zero value of the type
// when the declaration gives none. Actions rebind it with name := value.

// VarDecl is a declared variable.
type VarDecl struct {
	Name string
	Typ  types.BasicTy // TyInt, TyString or TyBool
	Init any           // the initial value: an int, a string or a bool
}

// VarTable holds the variables of a program in declaration order. A table is nil for a grammar
// without variables.
type VarTable struct {
	List   []*VarDecl
	byName map[string]*VarDecl
	// slots maps a name to its position in List once the table is large enough that a scan of
	// List is slower than a map lookup.
	slots map[string]int
}

// slotMapMin is the number of variables from which Index uses a map.
const slotMapMin = 8

// All returns the variables in declaration order.
func (t *VarTable) All() []*VarDecl {
	if t == nil {
		return nil
	}
	return t.List
}

// Lookup returns the declaration of the variable name, or nil.
func (t *VarTable) Lookup(name string) *VarDecl {
	if t == nil {
		return nil
	}
	return t.byName[name]
}

// Add appends a variable.
func (t *VarTable) Add(d *VarDecl) {
	t.List = append(t.List, d)
	t.byName[d.Name] = d
	if len(t.List) >= slotMapMin {
		if t.slots == nil {
			t.slots = make(map[string]int, len(t.List))
			for i, v := range t.List {
				t.slots[v.Name] = i
			}
		} else {
			t.slots[d.Name] = len(t.List) - 1
		}
	}
}

// Index returns the slot of the variable name, its position in declaration order, or -1.
func (t *VarTable) Index(name string) int {
	if t == nil {
		return -1
	}
	if t.slots != nil {
		if i, ok := t.slots[name]; ok {
			return i
		}
		return -1
	}
	for i, d := range t.List {
		if d.Name == name {
			return i
		}
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
		if d := t.Lookup(name); d != nil {
			vals[i] = d.Init
		}
	}
	return vals
}

// NewVarTable returns an empty variable table.
func NewVarTable() *VarTable {
	return &VarTable{byName: map[string]*VarDecl{}}
}

// ZeroValue returns the initial value of a variable of type t that has no initializer.
func ZeroValue(t types.BasicTy) any {
	switch t {
	case types.TyInt:
		return 0
	case types.TyString:
		return ""
	}
	return false
}

// ValueType returns the variable type of the value v, an int, a string or a bool.
func ValueType(v any) types.BasicTy {
	switch v.(type) {
	case int:
		return types.TyInt
	case string:
		return types.TyString
	}
	return types.TyBool
}

// ModuleVarTable returns the variable table that the module m declares, or nil when it declares
// none. It reads back the table that the bytecode compiler writes.
func ModuleVarTable(m *bytecode.Module) *VarTable {
	var t *VarTable
	for _, v := range m.Vars {
		d := &VarDecl{Name: m.Strings[v.Name]}
		switch v.Type {
		case bytecode.VarInt:
			d.Typ, d.Init = types.TyInt, v.Int
		case bytecode.VarString:
			d.Typ, d.Init = types.TyString, m.Strings[v.Str]
		default:
			d.Typ, d.Init = types.TyBool, v.Int == 1
		}
		if t == nil {
			t = NewVarTable()
		}
		t.Add(d)
	}
	return t
}
