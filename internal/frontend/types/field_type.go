package types

import "zeroparse.dev/zp/internal/runtime/program"

// FieldType is the run-time form of a declared struct field type (program.FieldType); the
// checker makes it from a resolved type (NewFieldType).
type FieldType = program.FieldType

// The kinds of a FieldType (program.FtInt and the others).
const (
	FtInt        = program.FtInt
	FtString     = program.FtString
	FtBool       = program.FtBool
	FtNil        = program.FtNil
	FtNode       = program.FtNode
	FtNamed      = program.FtNamed
	FtStringNode = program.FtStringNode
	FtError      = program.FtError
	FtMissing    = program.FtMissing
	FtList       = program.FtList
	FtOptional   = program.FtOptional
	FtJoin       = program.FtJoin
	FtNodeOf     = program.FtNodeOf
	FtTuple      = program.FtTuple
	FtTop        = program.FtTop
)

// ParseFieldType is program.ParseFieldType.
func ParseFieldType(text string, types map[string]bool) (*FieldType, error) {
	return program.ParseFieldType(text, types)
}

// NewFieldType converts a resolved declared type. It returns nil for any, which every value fits.
func NewFieldType(t Ty) *FieldType {
	var f *FieldType
	switch t := t.(type) {
	case BasicTy:
		switch t {
		case TyInt:
			f = &FieldType{Kind: FtInt}
		case TyString:
			f = &FieldType{Kind: FtString}
		case TyBool:
			f = &FieldType{Kind: FtBool}
		case TyNil:
			f = &FieldType{Kind: FtNil}
		case TyNode:
			f = &FieldType{Kind: FtNode}
		case TyErr:
			f = &FieldType{Kind: FtError}
		default:
			return nil
		}
	case NodeTy:
		// A declared type has no captures. node[_] and node[string] have their own kinds, which
		// take one step to check. A named node type is checked by its name.
		switch {
		case t.Name != "":
			f = &FieldType{Kind: FtNamed, Name: t.Name}
		case len(t.Caps) > 0:
			return nil
		case t.Value == TyTop || t.Value == TyAny:
			f = &FieldType{Kind: FtNode}
		case t.Value == TyString:
			f = &FieldType{Kind: FtStringNode}
		default:
			f = &FieldType{Kind: FtNodeOf, Elem: topFieldType(t.Value)}
		}
	case TupleTy:
		f = &FieldType{Kind: FtTuple, Alts: []*FieldType{}}
		for _, e := range t.Elems {
			f.Alts = append(f.Alts, topFieldType(e))
		}
	case NamedTy:
		switch {
		case t == TyMatch:
			f = &FieldType{Kind: FtStringNode}
		case t == TyError:
			f = &FieldType{Kind: FtError}
		case t == TyMissing:
			f = &FieldType{Kind: FtMissing}
		case t.Kind == 'c':
			f = &FieldType{Kind: FtNode}
		default:
			f = &FieldType{Kind: FtNamed, Name: t.Name}
		}
	case ListTy:
		// A list of _ or any only has to be a list, which takes one step to check whatever its
		// length.
		f = &FieldType{Kind: FtList, Elem: topFieldType(t.Elem)}
	case OptTy:
		e := NewFieldType(t.Elem)
		if e == nil {
			return nil
		}
		f = &FieldType{Kind: FtOptional, Elem: e}
	case JoinTy:
		f = &FieldType{Kind: FtJoin}
		for _, a := range t.Alts {
			e := NewFieldType(a)
			if e == nil {
				return nil
			}
			f.Alts = append(f.Alts, e)
		}
	default:
		return nil
	}
	f.Text = t.String()
	return f
}

// topFieldType is newFieldType for a part of a type, where _ and any are the type ftTop.
func topFieldType(t Ty) *FieldType {
	if f := NewFieldType(t); f != nil {
		return f
	}
	return &FieldType{Kind: FtTop, Text: t.String()}
}
