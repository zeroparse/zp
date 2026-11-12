package compile

import "strings"

// HasZero is Checker.HasZero.
func (p *Program) HasZero(t Ty) bool { return p.Checker().HasZero(t) }

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
	k := &TypeResolver{Prog: p.Checker()}
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
// tuple of the zero values of the elements, written one after the other; {Name} for the struct type
// Name with no field set (a runtime does not build the zero values of its fields, which a read of one of
// them builds from the descriptor of the field); and NoZeroDesc for a type that has none (HasZero).
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
