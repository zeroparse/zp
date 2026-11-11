package types

import (
	"fmt"
	"math/rand/v2"
	"reflect"
	"slices"
	"testing"
)

// vjoinReference is Vjoin as it was written when duplicates were found by printing every type with
// fmt: the specification that the faster Vjoin must keep. The empty list type []_|_ below another
// type is removed after the duplicates, by the same rule as in Vjoin.
func vjoinReference(ts ...Ty) Ty {
	var alts []Ty
	seen := map[string]bool{}
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
		key := fmt.Sprintf("%T %s", t, t)
		if t == nil || t == TyBottom || seen[key] {
			return
		}
		seen[key] = true
		alts = append(alts, t)
	}
	for _, t := range ts {
		add(t)
	}
	if slices.Contains(alts, Ty(TyAny)) {
		return TyAny
	}
	if slices.Contains(alts, Ty(TyTop)) {
		return TyTop
	}
	if len(alts) > 1 {
		alts = dropEmptyBelow(alts)
	}
	if optional {
		if len(alts) == 0 {
			return TyNil // *_|_
		}
		return OptTy{vjoinReference(alts...)}
	}
	switch len(alts) {
	case 0:
		return TyBottom
	case 1:
		return alts[0]
	}
	return JoinTy{alts}
}

// randomTy returns a random type of at most the given depth. The types are drawn from a few names
// so that duplicates, and types that differ only where their text does not show it, are common.
func randomTy(r *rand.Rand, depth int) Ty {
	leaves := []Ty{TyString, TyTop, TyBottom, TyAny, TyNil, TyNode, TyErr, TyMatch, TyError, TyMissing,
		NamedTy{"P", 's'}, NamedTy{"Q", 's'}, RuleTy{"P"}, RuleTy{"Q"}, TyStringNode, nil}
	if depth == 0 || r.IntN(4) == 0 {
		return leaves[r.IntN(len(leaves))]
	}
	list := func(max int) []Ty {
		out := make([]Ty, r.IntN(max+1))
		for i := range out {
			out[i] = randomTy(r, depth-1)
		}
		return out
	}
	switch r.IntN(8) {
	case 0:
		return TupleTy{list(3)}
	case 1:
		var caps []CapIndex
		for i := range r.IntN(3) {
			caps = append(caps, CapIndex{Name: string(rune('a' + r.IntN(2))), Paths: [][]int{{i, r.IntN(2)}}, Side: r.IntN(2) == 0,
				T: randomTy(r, depth-1), Maybe: r.IntN(2) == 0})
		}
		n := NodeTy{Value: randomTy(r, depth-1), Caps: caps}
		if r.IntN(5) == 0 {
			n.Name = "N"
		}
		if n.Value == nil {
			n.Value = TyTop
		}
		return n
	case 2:
		return JoinTy{list(3)}
	case 3:
		return OptTy{randomTy(r, depth-1)}
	case 4:
		return ListTy{randomTy(r, depth-1)}
	case 5:
		return RecordTy{map[string]Ty{"x": TyString, "y": randomTy(r, depth-1)}}
	case 6:
		return NodeTy{Value: TupleTy{list(2)}}
	}
	return JoinTy{list(2)}
}

// safeString returns t.String(), or false if it panics (a nil part, which neither version prints).
func safeString(t Ty) (s string, ok bool) {
	defer func() { ok = recover() == nil }()
	return t.String(), true
}

func TestAppendTyTextIsString(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	checked := 0
	for range 20000 {
		ty := randomTy(r, 4)
		if ty == nil {
			continue
		}
		want, ok := safeString(ty)
		if !ok {
			continue
		}
		if got := string(AppendText(nil, ty)); got != want {
			t.Fatalf("%T: AppendText %q, String %q", ty, got, want)
		}
		checked++
	}
	if checked < 5000 {
		t.Fatalf("only %d types checked", checked)
	}
}

// TestVjoinKeepsTheDuplicatesItRemoved checks Vjoin against the version that printed every type
// with fmt, on random lists of types.
func TestVjoinKeepsTheDuplicatesItRemoved(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))
	checked, joins := 0, 0
	for range 40000 {
		// Most lists are short; some are long enough for the map of dedupTys.
		ts := make([]Ty, r.IntN(6))
		if r.IntN(10) == 0 {
			ts = make([]Ty, 30+r.IntN(30))
		}
		for i := range ts {
			ts[i] = randomTy(r, 3)
			if _, ok := safeString(ts[i]); !ok && ts[i] != nil {
				ts[i] = TyString
			}
		}
		// Printing a type with a nil part panics in both versions; skip such lists.
		want, wantPanic := join(vjoinReference, ts)
		got, gotPanic := join(Vjoin, ts)
		if wantPanic != gotPanic {
			t.Fatalf("panics differ for %v", ts)
		}
		if wantPanic {
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("Vjoin(%v) = %#v, want %#v", ts, got, want)
		}
		checked++
		if _, ok := want.(JoinTy); ok {
			joins++
		}
	}
	if checked < 10000 || joins < 1000 {
		t.Fatalf("%d lists checked, %d joins: too few", checked, joins)
	}
}

func join(f func(...Ty) Ty, ts []Ty) (out Ty, panicked bool) {
	defer func() { panicked = recover() != nil }()
	return f(ts...), false
}

// TestVjoinDuplicates pins what counts as a duplicate: the same Go type with the same text.
func TestVjoinDuplicates(t *testing.T) {
	a := NodeTy{Value: TyString, Caps: []CapIndex{{Name: "x", Paths: [][]int{{0}}}}}
	b := NodeTy{Value: TyString, Caps: []CapIndex{{Name: "x", Paths: [][]int{{0}}, Maybe: true}}} // the same text
	if got := Vjoin(a, b); !reflect.DeepEqual(got, a) {
		t.Errorf("types that print alike are not joined to the first: %#v", got)
	}
	if got := Vjoin(RuleTy{"P"}, NamedTy{"P", 's'}); !reflect.DeepEqual(got, JoinTy{[]Ty{RuleTy{"P"}, NamedTy{"P", 's'}}}) {
		t.Errorf("a rule and a type of one name are joined as one: %#v", got)
	}
}
