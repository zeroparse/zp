package types

import (
	"reflect"
	"slices"
	"strconv"
	"strings"
)

// This file holds the types of the value model, in which a rule produces an ordinary typed value
// and the syntax tree is built from one generic node[T, C].

// Types of the value model, beside BasicTy, NamedTy, ListTy and OptTy.
const (
	TyTop    BasicTy = "_"   // every value
	TyBottom BasicTy = "_|_" // no value
	TyErr    BasicTy = "error"
)

// TupleTy is a tuple (T, U, ...).
type TupleTy struct{ Elems []Ty }

// String returns the type as (T, U, ...), with a trailing comma for one element.
func (t TupleTy) String() string {
	parts := make([]string, len(t.Elems))
	for i, e := range t.Elems {
		parts[i] = e.String()
	}
	if len(parts) == 1 {
		return "(" + parts[0] + ",)"
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

// CapIndex locates a capture in the value of a node. In a path, the first index is an element of
// the node's tuple, and each further index an element of the tuple in the node at the previous
// one. A capture in a positive lookahead has no element: its value is kept beside the tuple, which
// side records. A capture in choice alternatives of different shapes has a location for each
// shape; the alternative that matched selects one at run time. A capture inside an optional
// expression or in only some alternatives may not be made; Maybe records that, and Set is then its
// type when it is made, while t also admits nil.
type CapIndex struct {
	Name  string
	Paths [][]int
	Side  bool
	T     Ty
	Maybe bool
	Set   Ty
}

// NodeTy is node[T, C]: a node whose Value has type value, with the captures caps. A named node
// type (type N = node[T]) has the name N and no captures: it is a distinct type over node[T].
type NodeTy struct {
	Value Ty
	Caps  []CapIndex // sorted by name
	Name  string
}

// Underlying returns the unnamed node type of a named node type, and t itself otherwise.
func (t NodeTy) Underlying() NodeTy {
	t.Name = ""
	return t
}

// String returns the name of a named node type, and the value-model term of another node type.
func (t NodeTy) String() string {
	if t.Name != "" {
		return t.Name
	}
	if t.Value == TyTop && len(t.Caps) == 0 {
		return "node[_]"
	}
	if len(t.Caps) == 0 {
		return "node[" + t.Value.String() + "]"
	}
	return "node[" + t.Value.String() + ", " + t.CapsText() + "]"
}

// CapsText is the text of the captures of t, as in node[T, {"name": 0.1}]. A name captured
// several times in one match is listed once, with the locations of all its captures.
func (t NodeTy) CapsText() string {
	var parts []string
	at := map[string]int{}
	for _, c := range t.Caps {
		var paths []string
		for _, path := range c.Paths {
			idx := make([]string, len(path))
			for k, p := range path {
				idx[k] = strconv.Itoa(p)
			}
			paths = append(paths, strings.Join(idx, "."))
		}
		if c.Side {
			paths = append(paths, "-")
		}
		if i, ok := at[c.Name]; ok {
			parts[i] += " | " + strings.Join(paths, " | ")
			continue
		}
		at[c.Name] = len(parts)
		parts = append(parts, strconv.Quote(c.Name)+": "+strings.Join(paths, " | "))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// RuleTy is the value type of a rule, referred to by the rule's name.
type RuleTy struct{ Name string }

// String returns the rule's name.
func (t RuleTy) String() string { return t.Name }

// Vjoin is the join of types in the value model: nested joins are flattened, duplicates and
// _|_ removed, and a join with _ is _. Since T <= *T and nil <= *T, a join with an optional
// argument or nil is the optional of the join of the other arguments without their *; *_|_ is
// nil, the optional type whose only value is nil. A type that holds the empty list []_|_ where
// another argument holds a list is removed, since []_|_ <= []T (belowByEmpty). A join with a
// value not known yet (tyAny) is not known either.
func Vjoin(ts ...Ty) Ty {
	var leaves []Ty
	optional := false
	var add func(t Ty)
	add = func(t Ty) {
		switch u := t.(type) {
		case JoinTy:
			for _, a := range u.Alts {
				add(a)
			}
			return
		case OptTy:
			optional = true
			add(u.Elem)
			return
		case BasicTy:
			if u == TyNil {
				optional = true
				return
			}
		}
		if t == nil || t == TyBottom {
			return
		}
		leaves = append(leaves, t)
	}
	for _, t := range ts {
		add(t)
	}
	alts := leaves
	if len(leaves) > 1 {
		alts = dropEmptyBelow(dedupTys(leaves))
	}
	if slices.Contains(alts, Ty(TyAny)) {
		return TyAny
	}
	if slices.Contains(alts, Ty(TyTop)) {
		return TyTop
	}
	if optional {
		if len(alts) == 0 {
			return TyNil // *_|_, whose only value is nil
		}
		return OptTy{Vjoin(alts...)}
	}
	switch len(alts) {
	case 0:
		return TyBottom
	case 1:
		return alts[0]
	}
	return JoinTy{alts}
}

// dropEmptyBelow returns ts without the types that hold the empty list type []_|_ and are below
// another type of ts, such as node[[]_|_] beside node[[]node[string]]: []_|_ is the least list
// type, whose only value is the empty list.
func dropEmptyBelow(ts []Ty) []Ty {
	if !slices.ContainsFunc(ts, holdsEmptyList) {
		return ts
	}
	var out []Ty
next:
	for i, t := range ts {
		if holdsEmptyList(t) {
			for j, u := range ts {
				if j != i && belowByEmpty(t, u) {
					continue next
				}
			}
		}
		out = append(out, t)
	}
	return out
}

// holdsEmptyList reports whether t has the list type []_|_ in a list, a node, a tuple or an
// optional type.
func holdsEmptyList(t Ty) bool {
	switch t := t.(type) {
	case ListTy:
		return t.Elem == TyBottom || holdsEmptyList(t.Elem)
	case NodeTy:
		return t.Name == "" && holdsEmptyList(t.Value)
	case TupleTy:
		return slices.ContainsFunc(t.Elems, holdsEmptyList)
	case OptTy:
		return holdsEmptyList(t.Elem)
	}
	return false
}

// belowByEmpty reports whether t is u with []_|_ in place of some of its list types, so that the
// values of t are values of u. It compares the other parts by their text.
func belowByEmpty(t, u Ty) bool {
	switch t := t.(type) {
	case ListTy:
		ul, ok := u.(ListTy)
		return ok && (t.Elem == TyBottom || belowByEmpty(t.Elem, ul.Elem))
	case NodeTy:
		un, ok := u.(NodeTy)
		return ok && t.Name == "" && un.Name == "" && SameCaps(t.Caps, un.Caps) && belowByEmpty(t.Value, un.Value)
	case TupleTy:
		ut, ok := u.(TupleTy)
		if !ok || len(t.Elems) != len(ut.Elems) {
			return false
		}
		for i := range t.Elems {
			if !belowByEmpty(t.Elems[i], ut.Elems[i]) {
				return false
			}
		}
		return true
	case OptTy:
		uo, ok := u.(OptTy)
		return ok && belowByEmpty(t.Elem, uo.Elem)
	}
	return reflect.TypeOf(t) == reflect.TypeOf(u) && string(AppendText(nil, t)) == string(AppendText(nil, u))
}

// tyKey identifies a type for the removal of duplicates from a join: two types are the same if
// they have the same Go type and print the same text. Rule and type names are separate namespaces,
// so the key includes the kind of type.
type tyKey struct {
	kind reflect.Type
	text string
}

// dedupTys returns ts without the types that an earlier type equals in its tyKey.
func dedupTys(ts []Ty) []Ty {
	if len(ts) > 32 {
		return dedupTysMap(ts)
	}
	// A few types: compare each with the earlier ones of its Go type, printing a type only when
	// there is one.
	var out []Ty
	var kinds [32]reflect.Type
	var texts [32]string
	var printed [32]bool
	var buf []byte
	text := func(i int, t Ty) string {
		if !printed[i] {
			buf = AppendText(buf[:0], t)
			texts[i], printed[i] = string(buf), true
		}
		return texts[i]
	}
	for _, t := range ts {
		k := reflect.TypeOf(t)
		dup := false
		var s string
		have := false
		for i, o := range out {
			if kinds[i] != k {
				continue
			}
			if !have {
				buf = AppendText(buf[:0], t)
				s, have = string(buf), true
			}
			if text(i, o) == s {
				dup = true
				break
			}
		}
		if dup {
			continue
		}
		kinds[len(out)] = k
		if have {
			texts[len(out)], printed[len(out)] = s, true
		}
		out = append(out, t)
	}
	return out
}

// dedupTysMap is dedupTys for many types.
func dedupTysMap(ts []Ty) []Ty {
	seen := make(map[tyKey]struct{}, len(ts))
	var out []Ty
	var buf []byte
	for _, t := range ts {
		buf = AppendText(buf[:0], t)
		k := tyKey{reflect.TypeOf(t), string(buf)}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, t)
	}
	return out
}

// AppendText appends t.String() to dst. It prints the composite types without building the text
// of each part as a string of its own, which dominates the cost of Vjoin on the nested types of
// an Operators Rule.
func AppendText(dst []byte, t Ty) []byte {
	switch u := t.(type) {
	case TupleTy:
		dst = append(dst, '(')
		for i, e := range u.Elems {
			if i > 0 {
				dst = append(dst, ", "...)
			}
			dst = AppendText(dst, e)
		}
		if len(u.Elems) == 1 {
			dst = append(dst, ',')
		}
		return append(dst, ')')
	case NodeTy:
		switch {
		case u.Name != "":
			return append(dst, u.Name...)
		case u.Value == TyTop && len(u.Caps) == 0:
			return append(dst, "node[_]"...)
		case len(u.Caps) == 0:
			dst = append(dst, "node["...)
			dst = AppendText(dst, u.Value)
			return append(dst, ']')
		}
		dst = append(dst, "node["...)
		dst = AppendText(dst, u.Value)
		dst = append(dst, ", "...)
		dst = append(dst, u.CapsText()...)
		return append(dst, ']')
	case JoinTy:
		for i, a := range u.Alts {
			if i > 0 {
				dst = append(dst, " | "...)
			}
			dst = AppendText(dst, a)
		}
		return dst
	case OptTy:
		return appendTyTerm(append(dst, '*'), u.Elem)
	case ListTy:
		return appendTyTerm(append(dst, "[]"...), u.Elem)
	}
	return append(dst, t.String()...)
}

// appendTyTerm appends typeTerm(t) to dst.
func appendTyTerm(dst []byte, t Ty) []byte {
	if _, ok := t.(JoinTy); ok {
		dst = append(dst, '(')
		dst = AppendText(dst, t)
		return append(dst, ')')
	}
	return AppendText(dst, t)
}

// Vopt is *t; an optional value of an optional type is not nested, and *_|_ is nil, the optional
// type whose only value is nil.
func Vopt(t Ty) Ty {
	if t == TyTop {
		return t
	}
	return Vjoin(OptTy{t})
}

// TyStringNode is node[string], the type of a match.
var TyStringNode = NodeTy{Value: TyString}

// WalkTy calls visit on t and on every type it is made of, including the types of the captures of
// a node type.
func WalkTy(t Ty, visit func(Ty)) {
	visit(t)
	switch u := t.(type) {
	case TupleTy:
		for _, e := range u.Elems {
			WalkTy(e, visit)
		}
	case NodeTy:
		WalkTy(u.Value, visit)
		for _, c := range u.Caps {
			if c.T != nil {
				WalkTy(c.T, visit)
			}
			if c.Set != nil {
				WalkTy(c.Set, visit)
			}
		}
	case ListTy:
		WalkTy(u.Elem, visit)
	case OptTy:
		WalkTy(u.Elem, visit)
	case JoinTy:
		for _, a := range u.Alts {
			WalkTy(a, visit)
		}
	case RecordTy:
		for _, f := range u.Fields {
			WalkTy(f, visit)
		}
	}
}

// MapTy returns t with the types for which f reports a replacement replaced; it does not look
// inside a replacement. The captures of a node type are not mapped: they are not part of its text.
func MapTy(t Ty, f func(Ty) (Ty, bool)) Ty {
	if r, ok := f(t); ok {
		return r
	}
	switch u := t.(type) {
	case TupleTy:
		elems := make([]Ty, len(u.Elems))
		for i, e := range u.Elems {
			elems[i] = MapTy(e, f)
		}
		return TupleTy{elems}
	case NodeTy:
		u.Value = MapTy(u.Value, f)
		return u
	case ListTy:
		return ListTy{MapTy(u.Elem, f)}
	case OptTy:
		return OptTy{MapTy(u.Elem, f)}
	case JoinTy:
		alts := make([]Ty, len(u.Alts))
		for i, a := range u.Alts {
			alts[i] = MapTy(a, f)
		}
		return JoinTy{alts}
	case RecordTy:
		fields := make(map[string]Ty, len(u.Fields))
		for k, e := range u.Fields {
			fields[k] = MapTy(e, f)
		}
		return RecordTy{fields}
	}
	return t
}

// FromLegacy maps a legacy type (types.go) to the value model: reserved node kinds become nodes
// and the recovery kinds error. any stays any: it is assignable to every type, and its values are
// checked at run time.
func FromLegacy(t Ty) Ty {
	switch t := t.(type) {
	case nil:
		return TyTop
	case BasicTy:
		switch t {
		case TyNever:
			return TyBottom
		case TyNode:
			return NodeTy{Value: TyTop}
		}
		return t
	case NamedTy:
		switch {
		case t == TyMatch:
			return TyStringNode
		case t == TyError || t == TyMissing:
			return TyErr
		case t.Kind == 'c':
			return NodeTy{Value: TyTop}
		}
		return t
	case ListTy:
		return ListTy{FromLegacy(t.Elem)}
	case OptTy:
		return Vopt(FromLegacy(t.Elem))
	case JoinTy:
		ts := make([]Ty, len(t.Alts))
		for i, a := range t.Alts {
			ts[i] = FromLegacy(a)
		}
		return Vjoin(ts...)
	case RecordTy:
		return NodeTy{Value: TyTop}
	}
	return TyTop
}

// SameCaps reports whether a and b locate the same captures (names, paths and sides).
func SameCaps(a, b []CapIndex) bool {
	return slices.EqualFunc(a, b, func(x, y CapIndex) bool {
		return x.Name == y.Name && x.Side == y.Side && slices.EqualFunc(x.Paths, y.Paths, func(p, q []int) bool { return slices.Equal(p, q) })
	})
}
