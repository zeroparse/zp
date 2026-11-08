package compile

import (
	"strings"

	"zeroparse.dev/zp/grammar"
)

// HasZero reports whether the type t has a zero value, the value that an unset field of a struct
// reads as (spec/language/types.md#zero-values). int, string and bool have 0, "" and false; a list
// has the empty list; an optional type and _ have nil; a tuple has the tuple of the zero values of its
// elements; a struct has the struct whose fields are all at their zero values, if every one has one.
// Every other type has none: a node type, error, missing, a join, a meet, _|_, and a struct with a
// field that leads back to the struct through fields that are not optional.
func (p *Program) HasZero(t Ty) bool { return p.hasZero(t, map[string]bool{}) }

func (p *Program) hasZero(t Ty, visiting map[string]bool) bool {
	switch t := t.(type) {
	case BasicTy:
		switch t {
		case TyInt, TyString, TyBool, TyNil, TyTop, TyAny:
			return true
		}
	case NamedTy:
		if t.Kind != 's' || visiting[t.Name] {
			return false
		}
		_, st := p.StructType(t.Name)
		if st == nil {
			return false
		}
		visiting[t.Name] = true
		defer delete(visiting, t.Name)
		k := &TypeResolver{Prog: p}
		for _, f := range DefFields(st) {
			if !p.hasZero(k.Resolve(f.Type), visiting) {
				return false
			}
		}
		return true
	case TupleTy:
		for _, e := range t.Elems {
			if !p.hasZero(e, visiting) {
				return false
			}
		}
		return true
	case ListTy, OptTy:
		return true
	case RecordTy:
		for _, f := range t.fields {
			if !p.hasZero(f, visiting) {
				return false
			}
		}
		return true
	}
	return false
}

// optionalOf writes the optional type of the declared type text, which a field without a zero value
// can declare instead.
func optionalOf(text string) string {
	if strings.ContainsAny(text, "|&") {
		return "*(" + text + ")"
	}
	return "*" + text
}

// unsetFields reports the fields of the struct st that a constructor of it leaves unset although
// their types have no zero value.
func (v *ValueInference) unsetFields(t *grammar.New, st *grammar.TypeDef) {
	set := map[string]bool{}
	for _, fi := range t.Fields {
		set[fi.Name] = true
	}
	for _, f := range DefFields(st) {
		if set[f.Name] {
			continue
		}
		ft := v.resolve(f.Type)
		if ft == TyBottom || v.Prog.HasZero(ft) {
			continue
		}
		text := grammar.FormatType(f.Type)
		v.errorf(t.Diagnostic(), "field %s of %s has type %s, which has no zero value: set it in the constructor, or declare it as %s", f.Name, t.Type, text, optionalOf(text))
	}
}

// StructZero describes how the fields of a struct type read when a constructor left them unset: Names
// holds the field names in declaration order and Zeros the zero descriptor of each (ZeroDesc).
type StructZero struct {
	Names []string
	Zeros map[string]string
}

// SetStructZero makes z the zero descriptors of the fields of the struct type name, for a program
// that is loaded from a compiled module and has no type expressions to compute them from.
func (p *Program) SetStructZero(name string, z *StructZero) { p.structZeros.Store(name, z) }

// NoZeroDesc is the zero descriptor of a type that has no zero value.
const NoZeroDesc = "-"

// StructZeroOf returns the zero descriptors of the fields of the struct type name, or nil if name is
// not a struct type. The result is shared and must not be changed.
func (p *Program) StructZeroOf(name string) *StructZero {
	if z, ok := p.structZeros.Load(name); ok {
		return z.(*StructZero)
	}
	_, st := p.StructType(name)
	if st == nil {
		return nil
	}
	k := &TypeResolver{Prog: p}
	z := &StructZero{Zeros: map[string]string{}}
	for _, f := range DefFields(st) {
		z.Names = append(z.Names, f.Name)
		z.Zeros[f.Name] = p.ZeroDesc(k.Resolve(f.Type))
	}
	actual, _ := p.structZeros.LoadOrStore(name, z)
	return actual.(*StructZero)
}

// ZeroDesc returns the descriptor of the zero value of the type t, which every runtime builds the
// zero value from: i, s and b for 0, "" and false; n for nil; l for the empty list; (...) for the
// tuple of the zero values of the elements, written one after the other; {Name} for the zero value of
// the struct type Name; and NoZeroDesc for a type that has none (HasZero).
func (p *Program) ZeroDesc(t Ty) string {
	if !p.HasZero(t) {
		return NoZeroDesc
	}
	switch t := t.(type) {
	case BasicTy:
		switch t {
		case TyInt:
			return "i"
		case TyString:
			return "s"
		case TyBool:
			return "b"
		}
		return "n"
	case NamedTy:
		return "{" + t.Name + "}"
	case TupleTy:
		var b strings.Builder
		b.WriteByte('(')
		for _, e := range t.Elems {
			b.WriteString(p.ZeroDesc(e))
		}
		b.WriteByte(')')
		return b.String()
	case ListTy:
		return "l"
	}
	return "n"
}
