// Package types holds the type model of the grammar checker: the types that the checker infers
// and checks rule values, captures and value expressions with, and the operations on them that
// need no grammar (joins, printing, walking).
package types

import (
	"sort"
	"strings"

	"zeroparse.dev/zp/core/impl"
)

// Ty is a type used in type checking.
type Ty interface{ String() string }

// BasicTy is a built-in type.
type BasicTy string

// The basic types.
const (
	TyAny    BasicTy = "unknown" // type not known statically (compatible with every type); no type can be written to mean it
	TyNever  BasicTy = "_|_"     // no value, and the initial value for inference
	TyInt    BasicTy = "int"
	TyString BasicTy = "string"
	TyBool   BasicTy = "bool"
	TyNil    BasicTy = "nil"
	TyNode   BasicTy = "node" // any node
)

// String returns the name of the type; TyNode prints as node[_].
func (t BasicTy) String() string {
	if t == TyNode {
		return "node[_]" // node without a type argument is not a type
	}
	return string(t)
}

// NamedTy is a named node type.
type NamedTy struct {
	Name string
	Kind byte // 's': struct, 'c': reserved node kind
}

// String returns the name of a struct or Missing, and the value-model term of another reserved
// node kind (node[string], node, error), so that diagnostics name the types a grammar can write.
func (t NamedTy) String() string {
	if t.Kind == 'c' && t != TyMissing {
		return FromLegacy(t).String()
	}
	return t.Name
}

// The reserved node kinds as named types.
var (
	TyMatch   = NamedTy{coreimpl.TypeString, 'c'}
	TySeq     = NamedTy{coreimpl.TypeTuple, 'c'}
	TyList    = NamedTy{coreimpl.TypeList, 'c'}
	TyError   = NamedTy{coreimpl.TypeError, 'c'}
	TyMissing = NamedTy{coreimpl.TypeMissing, 'c'}
)

// ListTy is the list type []Elem.
type ListTy struct{ Elem Ty }

// String returns the type as []T.
func (t ListTy) String() string { return "[]" + typeTerm(t.Elem) }

// OptTy is the optional type *Elem.
type OptTy struct{ Elem Ty }

// String returns the type as *T.
func (t OptTy) String() string { return "*" + typeTerm(t.Elem) }

// JoinTy is the join of the types Alts.
type JoinTy struct{ Alts []Ty }

// String returns the alternatives separated by |.
func (t JoinTy) String() string {
	parts := make([]string, len(t.Alts))
	for i, a := range t.Alts {
		parts[i] = a.String()
	}
	return strings.Join(parts, " | ")
}

// RecordTy is the type of a tuple node whose fields are its captures. Diagnostics print it as
// node{k: T, ...}.
type RecordTy struct{ Fields map[string]Ty }

// String returns the type as node{k: T, ...}, with the fields sorted by name.
func (t RecordTy) String() string {
	names := make([]string, 0, len(t.Fields))
	for n := range t.Fields {
		names = append(names, n)
	}
	sort.Strings(names)
	parts := make([]string, len(names))
	for i, n := range names {
		parts[i] = n + ": " + t.Fields[n].String()
	}
	return "node{" + strings.Join(parts, ", ") + "}"
}

func typeTerm(t Ty) string {
	if _, ok := t.(JoinTy); ok {
		return "(" + t.String() + ")"
	}
	return t.String()
}

// IsNode reports whether t is a node type.
func IsNode(t Ty) bool {
	switch t := t.(type) {
	case NamedTy, ListTy, RecordTy:
		return true
	case BasicTy:
		return t == TyNode || t == TyAny || t == TyNever
	case NodeTy:
		return t.Name != "" // a named node type has a Go type of its own
	case JoinTy:
		for _, a := range t.Alts {
			if !IsNode(a) {
				return false
			}
		}
		return true
	}
	return false
}
