package conform

import (
	"fmt"
	"strings"
)

// The cast table: the check of a value against a type (the composition conforms and the ones it calls), on
// the values and the types of one fixture, and what each check returns and how many steps it charges. A type
// and a value are written in a small notation that each target builds its own from; the tests that do are
// the ones the mutants of the printed code name.
//
// A type is a kind letter of ops.FtInt and the like with the text that follows it: i, s, b, z, n, S, E, M and
// _; N:name for a struct type or a named node type; l<T> for a list, t<T,T...> for a tuple, o<T> for an optional
// type, u<T,T...> for a join and v<T> for a node whose value is of type T.
//
// A value is nil, an int (1), a string (s), a bool (true), a string node (text), a tuple node of two string
// nodes (tuple), a list node of two string nodes (rep), an error value (err), the error value of a missing
// token (missing), a struct of type S or T (S, T), a box of the int 1 (box), a list value [v,v...] or a tuple
// value (v,v...) of values, or a nil node (nilnode) that a list may hold where it holds nil.

// CastType is a type of a cast: its Kind is a letter of ops.FtInt and the like, Name the name of the struct type or
// the named node type of the kind N, Elem the element type, and Alts the element types or the alternatives.
type CastType struct {
	Kind byte
	Name string
	Elem *CastType
	Alts []*CastType
}

// CastValue is a value of a cast: its Kind is what the notation names (nil, int, string, bool, text, tuple, rep,
// err, missing, struct, box, list, tupleValue, nilnode), Name the type name of a struct, and Elems the elements of a list
// value or a tuple value.
type CastValue struct {
	Kind  string
	Name  string
	Elems []CastValue
}

// ParseCastType parses a type of the notation.
func ParseCastType(s string) *CastType {
	t, rest := parseCastType(s)
	if rest != "" {
		panic("conform: cast type " + s + ": text after the type: " + rest)
	}
	return t
}

func parseCastType(s string) (*CastType, string) {
	if s == "" {
		panic("conform: empty cast type")
	}
	t := &CastType{Kind: s[0]}
	rest := s[1:]
	switch t.Kind {
	case 'N':
		if rest == "" || rest[0] != ':' {
			panic("conform: cast type N needs :name")
		}
		end := strings.IndexAny(rest, ",>")
		if end < 0 {
			end = len(rest)
		}
		t.Name, rest = rest[1:end], rest[end:]
	case 'l', 'o', 'v', 't', 'u':
		if rest == "" || rest[0] != '<' {
			panic("conform: cast type " + s + " needs <...>")
		}
		rest = rest[1:]
		var parts []*CastType
		for {
			var p *CastType
			p, rest = parseCastType(rest)
			parts = append(parts, p)
			if rest[0] == ',' {
				rest = rest[1:]
				continue
			}
			if rest[0] != '>' {
				panic("conform: cast type: want , or > at " + rest)
			}
			rest = rest[1:]
			break
		}
		if t.Kind == 't' || t.Kind == 'u' {
			t.Alts = parts
		} else {
			t.Elem = parts[0]
		}
	}
	return t, rest
}

// ParseCastValue parses a value of the notation.
func ParseCastValue(s string) CastValue {
	v, rest := parseCastValue(s)
	if rest != "" {
		panic("conform: cast value " + s + ": text after the value: " + rest)
	}
	return v
}

func parseCastValue(s string) (CastValue, string) {
	list := func(open, close byte, kind string) (CastValue, string) {
		v := CastValue{Kind: kind}
		rest := s[1:]
		for rest[0] != close {
			var e CastValue
			e, rest = parseCastValue(rest)
			v.Elems = append(v.Elems, e)
			if rest[0] == ',' {
				rest = rest[1:]
			}
		}
		return v, rest[1:]
	}
	switch s[0] {
	case '[':
		return list('[', ']', "list")
	case '(':
		return list('(', ')', "tupleValue")
	}
	end := strings.IndexAny(s, ",])")
	if end < 0 {
		end = len(s)
	}
	word, rest := s[:end], s[end:]
	switch word {
	case "nil", "text", "tuple", "rep", "err", "missing", "box", "nilnode":
		return CastValue{Kind: word}, rest
	case "1":
		return CastValue{Kind: "int"}, rest
	case "s":
		return CastValue{Kind: "string"}, rest
	case "true":
		return CastValue{Kind: "bool"}, rest
	case "S", "T":
		return CastValue{Kind: "struct", Name: word}, rest
	}
	panic("conform: cast value " + word)
}

// CastStructs are the struct types of the fixture, each with no field.
var CastStructs = []string{"S", "T"}

// CastCase checks the value Value against the type Type. It returns Want: whether the value is of the type,
// and the steps the check charges; Only, if not empty, is the one target that can build the value.
type CastCase struct {
	Type, Value string
	Ok          bool
	Steps       int
	Only        string
}

// CastCases are the checks of the cast table: every kind of type, the values that it takes and the ones that
// it does not, and the lists and tuples, alone and nested, whose elements the check examines in order up to
// the first that fails, and charges a step for each.
var CastCases = []CastCase{
	// The scalar types and nil.
	{"i", "1", true, 0, ""}, {"i", "s", false, 0, ""}, {"i", "true", false, 0, ""}, {"i", "nil", false, 0, ""},
	{"s", "s", true, 0, ""}, {"s", "1", false, 0, ""}, {"b", "true", true, 0, ""}, {"b", "1", false, 0, ""},
	{"z", "nil", true, 0, ""}, {"z", "1", false, 0, ""}, {"z", "nilnode", true, 0, ""},
	{"_", "1", true, 0, ""}, {"_", "nil", true, 0, ""}, {"_", "text", true, 0, ""},
	// A box stands for the value it holds.
	{"i", "box", true, 0, ""}, {"s", "box", false, 0, ""}, {"o<i>", "box", true, 0, ""},
	// An optional type and a join.
	{"o<i>", "nil", true, 0, ""}, {"o<i>", "1", true, 0, ""}, {"o<i>", "s", false, 0, ""}, {"o<N:S>", "nil", true, 0, ""},
	{"o<N:S>", "S", true, 0, ""}, {"o<N:S>", "T", false, 0, ""}, {"o<i>", "nilnode", true, 0, ""},
	{"u<i,s>", "1", true, 0, ""}, {"u<i,s>", "s", true, 0, ""}, {"u<i,s>", "true", false, 0, ""}, {"u<i,s>", "nil", false, 0, ""},
	{"u<N:S,N:T>", "T", true, 0, ""}, {"u<N:S,N:T>", "text", false, 0, ""}, {"u<l<i>,s>", "[1,1]", true, 2, ""},
	{"u<l<i>,l<s>>", "[s,s]", true, 3, ""}, {"u<l<i>,l<s>>", "[1,s]", false, 3, ""},
	// The types of nodes: a string, a tuple and a list node, an error and a struct.
	{"n", "text", true, 0, ""}, {"n", "tuple", true, 0, ""}, {"n", "rep", true, 0, ""}, {"n", "err", false, 0, ""},
	{"n", "S", false, 0, ""}, {"n", "[1]", false, 0, ""}, {"n", "1", false, 0, ""}, {"n", "nil", false, 0, ""}, {"n", "nilnode", false, 0, ""},
	{"S", "text", true, 0, ""}, {"S", "tuple", false, 0, ""}, {"S", "s", false, 0, ""}, {"S", "S", false, 0, ""},
	{"E", "err", true, 0, ""}, {"E", "text", false, 0, ""}, {"E", "1", false, 0, ""},
	{"M", "missing", true, 0, "engine"}, {"M", "err", false, 0, ""},
	{"N:S", "S", true, 0, ""}, {"N:T", "S", false, 0, ""}, {"N:S", "text", false, 0, ""}, {"N:S", "1", false, 0, ""}, {"N:S", "nil", false, 0, ""},
	{"N:S", "[]", false, 0, ""},
	// List values and tuple values: a list type takes a list value, and not a list node or a tuple value.
	{"l<i>", "[]", true, 0, ""}, {"l<i>", "[1]", true, 1, ""}, {"l<i>", "[1,1,1]", true, 3, ""}, {"l<i>", "[1,s]", false, 2, ""},
	{"l<i>", "[s,1]", false, 1, ""}, {"l<i>", "[1,1,s]", false, 3, ""}, {"l<_>", "[1,s]", true, 0, ""}, {"l<i>", "1", false, 0, ""},
	{"l<i>", "(1,1)", false, 0, ""}, {"l<i>", "rep", false, 0, ""}, {"l<i>", "nil", false, 0, ""}, {"l<o<i>>", "[1,nilnode]", true, 2, ""},
	{"l<S>", "[text,text]", true, 2, ""}, {"l<S>", "[text,tuple]", false, 2, ""}, {"l<N:S>", "[S,T]", false, 2, ""},
	{"l<l<i>>", "[[1],[1,1]]", true, 5, ""}, {"l<l<i>>", "[[1],[s]]", false, 4, ""}, {"l<l<i>>", "[[1],1]", false, 3, ""},
	{"l<l<i>>", "[[],[]]", true, 2, ""}, {"l<l<_>>", "[[1,s],[]]", true, 2, ""},
	{"t<i,s>", "(1,s)", true, 2, ""}, {"t<i,s>", "(1,1)", false, 2, ""}, {"t<i,s>", "(s,s)", false, 1, ""}, {"t<i,s>", "(1)", false, 0, ""},
	{"t<i,s>", "(1,s,true)", false, 0, ""}, {"t<i,s>", "[1,s]", false, 0, ""}, {"t<i,s>", "tuple", false, 0, ""}, {"t<i,s>", "nil", false, 0, ""},
	{"t<_,_>", "(1,s)", true, 2, ""}, {"t<i,l<i>>", "(1,[1,1])", true, 4, ""}, {"t<i,l<i>>", "(1,[1,s])", false, 4, ""},
	{"t<i,t<i,s>>", "(1,(1,s))", true, 4, ""}, {"t<i,t<i,s>>", "(1,(1,1))", false, 4, ""}, {"l<t<i,s>>", "[(1,s),(1,s)]", true, 6, ""},
	{"l<t<i,s>>", "[(1,s),(1,1)]", false, 6, ""}, {"t<i,o<i>>", "(1,nil)", true, 2, ""},
	// A node whose value has a type: a string node has a string, a tuple node and a list node their elements.
	{"v<s>", "text", true, 0, ""}, {"v<s>", "tuple", false, 0, ""}, {"v<s>", "rep", false, 0, ""}, {"v<s>", "1", false, 0, ""},
	{"v<_>", "text", true, 0, ""}, {"v<_>", "tuple", true, 0, ""}, {"v<_>", "rep", true, 0, ""}, {"v<_>", "err", false, 0, ""},
	{"v<_>", "S", false, 0, ""}, {"v<_>", "[1]", false, 0, ""},
	{"v<t<S,S>>", "tuple", true, 2, ""}, {"v<t<S,i>>", "tuple", false, 2, ""}, {"v<t<i,S>>", "tuple", false, 1, ""},
	{"v<t<S,S,S>>", "tuple", false, 0, ""}, {"v<t<S,S>>", "text", false, 0, ""}, {"v<t<S,S>>", "rep", false, 0, ""},
	{"v<t<S,S>>", "(text,text)", false, 0, ""},
	{"v<l<S>>", "rep", true, 2, ""}, {"v<l<i>>", "rep", false, 1, ""}, {"v<l<_>>", "rep", true, 0, ""}, {"v<l<S>>", "tuple", false, 0, ""},
	{"v<l<S>>", "text", false, 0, ""}, {"v<l<S>>", "[text,text]", false, 0, ""},
	{"v<o<s>>", "text", true, 0, ""}, {"v<o<s>>", "tuple", false, 0, ""}, {"v<o<l<S>>>", "rep", true, 2, ""},
	{"v<u<s,l<S>>>", "rep", true, 2, ""}, {"v<u<s,l<S>>>", "tuple", false, 0, ""}, {"v<u<s,l<S>>>", "text", true, 0, ""},
	{"v<u<l<i>,l<S>>>", "rep", true, 3, ""}, {"v<i>", "text", false, 0, ""}, {"v<N:S>", "text", false, 0, ""},
}

// RunCasts runs every case of the cast table on check, which builds the value of a case, checks it against
// the type, and returns whether it is of the type and the steps the check charged (not examined if the run
// is not controlled) in a target. only is the name of the target for the cases that only one target can
// run. It returns the cases whose result differs.
func RunCasts(target string, controlled bool, check func(t *CastType, v CastValue) (bool, int)) error {
	var bad []string
	for _, c := range CastCases {
		if c.Only != "" && c.Only != target {
			continue
		}
		ok, steps := check(ParseCastType(c.Type), ParseCastValue(c.Value))
		if ok != c.Ok || (controlled && steps != c.Steps) {
			bad = append(bad, fmt.Sprintf("%s: %s is %s: got %v with %d steps, want %v with %d", target, c.Value, c.Type, ok, steps, c.Ok, c.Steps))
		}
	}
	if len(bad) > 0 {
		return fmt.Errorf("cast table:\n%s", strings.Join(bad, "\n"))
	}
	return nil
}
