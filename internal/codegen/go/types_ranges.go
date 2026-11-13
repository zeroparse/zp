package gogen

import (
	"fmt"
	"strings"

	"zeroparse.dev/zp/internal/compile"
)

// The range of a list or a tuple that an action reads from a field of a struct is the range it had where it
// was made: an action that builds a struct from it gives the struct the range that covers it. The typed
// library keeps a struct's lists and tuples as Go slices and tuple structs, which have no range, so a struct
// of a parser that runs the typed library keeps the ranges of the lists and tuples of each field that some
// action reads in an unexported field next to it (zpr_Field), written where the constructor converts the
// value (tmk_) and read back where the field is read (TField, TFieldSpend). A field holds
//
//   - no range unless it is read and its type is a list or a tuple (rangeNone);
//   - the one range of its list or tuple if no element is a list or a tuple (rangeOne);
//   - the ranges of its list or tuple and of those inside it, in the order that a read builds them
//     (rangeAll): a list or tuple before its elements, a nil list not at all.

const (
	rangeNone = iota
	rangeOne
	rangeAll
)

// carrierTy reports whether a value of type x is a list or a tuple, which has a range of its own that a Go
// value does not keep.
func carrierTy(x compile.Ty) bool {
	switch x.(type) {
	case compile.ListTy, compile.TupleTy:
		return true
	}
	return false
}

// nestsCarrier reports whether a list or a tuple of type x holds another list or tuple, or a value that
// holds one, as an element.
func nestsCarrier(x compile.Ty) bool {
	switch x := x.(type) {
	case compile.ListTy:
		return carrierTy(x.Elem) || nestsCarrier(x.Elem)
	case compile.TupleTy:
		for _, e := range x.Elems {
			if carrierTy(e) || nestsCarrier(e) {
				return true
			}
		}
	}
	return false
}

// rangeKind returns what the field name of type x keeps of the ranges of its lists and tuples.
func (t *TypedGen) rangeKind(name string, x compile.Ty) int {
	switch {
	case !t.direct || !t.reads[name] || !carrierTy(x):
		return rangeNone
	case nestsCarrier(x):
		return rangeAll
	}
	return rangeOne
}

// rangeField returns the name of the unexported field that keeps the ranges of the field name.
func (t *TypedGen) rangeField(name string) string { return "zpr_" + t.fieldGo(name) }

// rangeDecls writes the unexported fields that keep the ranges of the lists and tuples of the fields of the
// struct name.
func (t *TypedGen) rangeDecls(b *strings.Builder, name string) {
	for _, f := range t.Info.Fields[name] {
		switch t.rangeKind(f.Name, f.T) {
		case rangeOne:
			fmt.Fprintf(b, "\t%s %s.Span\n", t.rangeField(f.Name), libAlias)
		case rangeAll:
			fmt.Fprintf(b, "\t%s []%s.Span\n", t.rangeField(f.Name), libAlias)
		}
	}
}

// rangeMethods writes the methods that let the parser package, which cannot name the unexported fields, set
// and copy the ranges of the struct name.
func (t *TypedGen) rangeMethods(b *strings.Builder, name string) {
	var set, cp strings.Builder
	for i, f := range t.Info.Fields[name] {
		switch t.rangeKind(f.Name, f.T) {
		case rangeOne:
			fmt.Fprintf(&set, "\tcase %d:\n\t\tv.%s = one\n", i, t.rangeField(f.Name))
		case rangeAll:
			fmt.Fprintf(&set, "\tcase %d:\n\t\tv.%s = all\n", i, t.rangeField(f.Name))
		default:
			continue
		}
		fmt.Fprintf(&cp, "\tv.%[1]s = from.%[1]s\n", t.rangeField(f.Name))
	}
	if set.Len() == 0 {
		return
	}
	fmt.Fprintf(b, "// TSetRanges sets the ranges that the list or tuple of field i had where it was made.\nfunc (v *%[1]s) TSetRanges(i int, one %[2]s.Span, all []%[2]s.Span) {\n\tswitch i {\n%[3]s\t}\n}\n\n", name, libAlias, set.String())
	fmt.Fprintf(b, "// TCopyRanges copies the ranges of the lists and tuples of from.\nfunc (v *%[1]s) TCopyRanges(from *%[1]s) {\n%[2]s}\n\n", name, cp.String())
}

// hasRanges reports whether the struct name keeps the ranges of any field.
func (t *TypedGen) hasRanges(name string) bool {
	for _, f := range t.Info.Fields[name] {
		if t.rangeKind(f.Name, f.T) != rangeNone {
			return true
		}
	}
	return false
}

// readField returns the Go expression that reads the field name of type x of the struct v (a Go expression
// of the struct), as a value of the typed library: back, or backSpend if spend, and with the ranges that the
// struct keeps for it if it keeps any.
func (t *TypedGen) readField(name string, x compile.Ty, v string, spend bool) string {
	expr := "v." + t.fieldGo(name)
	var cursor string
	switch t.rangeKind(name, x) {
	case rangeOne:
		cursor = fmt.Sprintf("One: v.%s", t.rangeField(name))
	case rangeAll:
		cursor = fmt.Sprintf("All: v.%s", t.rangeField(name))
	default:
		if spend {
			return t.backSpend(x, expr)
		}
		return t.back(x, expr)
	}
	return fmt.Sprintf("func() any { zc := &zpRangeCursor{%s}; return %s }()", cursor, t.backRanged(x, expr, "zc", spend))
}

// backRanged is back (backSpend if spend) for a list or a tuple whose range, and the range of each list and
// tuple in it, the cursor cur (a Go expression of type *zpRangeCursor) hands out, a list or tuple before its
// elements.
func (t *TypedGen) backRanged(x compile.Ty, v, cur string, spend bool) string {
	switch x := x.(type) {
	case compile.ListTy:
		step, elem := "", ""
		if spend {
			step = strings.ReplaceAll(t.G.step("p"), "\n", "; ")
		}
		elem = t.backRanged(x.Elem, "x", cur, spend)
		done := "zpListAt(kids, sp)"
		if spend {
			done = "zpListFieldAt(p, kids, sp)"
		}
		return fmt.Sprintf("func() any { if %[1]s == nil { return nil }; sp := %[2]s.Next(); kids := make([]any, len(%[1]s)); for i, x := range %[1]s { %[3]skids[i] = %[4]s }; return %[5]s }()", v, cur, step, elem, done)
	case compile.TupleTy:
		return t.backRangedHelper(x) + "(" + v + ", " + cur + ")"
	}
	if spend {
		return t.backSpend(x, v)
	}
	return t.back(x, v)
}

// rangedHelper writes the helper h of backRanged: the tuple value of a Go tuple.
func (t *TypedGen) rangedHelper(b *strings.Builder, h vhelper) {
	x := h.x.(compile.TupleTy)
	fmt.Fprintf(b, "func %s(v %s, zc *zpRangeCursor) any {\n", h.name, t.goType(x))
	if len(x.Elems) == 0 {
		fmt.Fprintf(b, "\treturn &%s.TSeq{Span: zc.Next(), Typ: %[1]s.TypeTupleValue}\n}\n\n", libAlias)
		return
	}
	// The range is taken before those of the elements.
	fmt.Fprintf(b, "\tsp := zc.Next()\n\treturn &%[1]s.TSeq{Span: sp, Typ: %[1]s.TypeTupleValue, Kids: []any{", libAlias)
	for i, e := range x.Elems {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(t.backRanged(e, fmt.Sprintf("v.E%d", i), "zc", false))
	}
	b.WriteString("}}\n}\n\n")
}

// rangeFunc returns the name of the function that appends to rs the ranges of the list or tuple value v of
// type x and of those in it, in the order that backRanged takes them.
func (t *TypedGen) rangeFunc(x compile.Ty) string {
	key := t.goType(x)
	if name, ok := t.rfuncs[key]; ok {
		return name
	}
	name := fmt.Sprintf("tr%d", len(t.rfuncs))
	t.rfuncs[key] = name
	t.rfuncList = append(t.rfuncList, listConv{name, x})
	return name
}

// rangeFuncs writes the functions that rangeFunc names, and any that they name.
func (t *TypedGen) rangeFuncs(b *strings.Builder) {
	if t.anyRanges() {
		b.WriteString("// trOne returns the range of the list or tuple value v that an action made, for the struct that is made of it\n// to keep, and the empty range at 0 for a value that holds none.\nfunc trOne(v any) Span {\n\tif n, _ := v.(*TSeq); n != nil {\n\t\treturn n.Span\n\t}\n\treturn Span{}\n}\n\n")
	}
	for i := 0; i < len(t.rfuncList); i++ {
		f := t.rfuncList[i]
		fmt.Fprintf(b, "// %s appends the ranges of the lists and tuples in the value v, a value of type %s.\nfunc %[1]s(v any, rs []Span) []Span {\n", f.name, f.elem)
		switch x := f.elem.(type) {
		case compile.ListTy:
			b.WriteString("\tif v == nil {\n\t\treturn rs\n\t}\n\tn, _ := v.(*TSeq)\n\tif n == nil {\n\t\treturn append(rs, Span{})\n\t}\n\trs = append(rs, n.Span)\n")
			if carrierTy(x.Elem) {
				fmt.Fprintf(b, "\tfor _, c := range n.Kids {\n\t\trs = %s(c, rs)\n\t}\n", t.rangeFunc(x.Elem))
			}
		case compile.TupleTy:
			nested := false
			for _, e := range x.Elems {
				nested = nested || carrierTy(e)
			}
			if nested {
				b.WriteString("\tn, _ := v.(*TSeq)\n\tvar sp Span\n\tvar kids []any\n\tif n != nil {\n\t\tsp, kids = n.Span, n.Kids\n\t}\n\trs = append(rs, sp)\n")
			} else {
				b.WriteString("\tn, _ := v.(*TSeq)\n\tvar sp Span\n\tif n != nil {\n\t\tsp = n.Span\n\t}\n\trs = append(rs, sp)\n")
			}
			for i, e := range x.Elems {
				if carrierTy(e) {
					fmt.Fprintf(b, "\t{\n\t\tvar c any\n\t\tif %d < len(kids) {\n\t\t\tc = kids[%[1]d]\n\t\t}\n\t\trs = %s(c, rs)\n\t}\n", i, t.rangeFunc(e))
				}
			}
		}
		b.WriteString("\treturn rs\n}\n\n")
	}
}

// rangeStatements returns the statements of the constructor of the struct name that keep the ranges of its
// fields, whose values the parameters f0, f1, ... hold.
func (t *TypedGen) rangeStatements(name string) string {
	var b strings.Builder
	for i, f := range t.Info.Fields[name] {
		switch t.rangeKind(f.Name, f.T) {
		case rangeOne:
			fmt.Fprintf(&b, "\tv.TSetRanges(%d, trOne(f%[1]d), nil)\n", i)
		case rangeAll:
			fmt.Fprintf(&b, "\tv.TSetRanges(%d, Span{}, %s(f%[1]d, nil))\n", i, t.rangeFunc(f.T))
		}
	}
	return b.String()
}

// anyRanges reports whether any struct keeps the ranges of a field.
func (t *TypedGen) anyRanges() bool {
	for _, name := range t.structs {
		if t.hasRanges(name) {
			return true
		}
	}
	return false
}

// rangeHelpers writes, into the types package, what a read of a field that keeps ranges uses: the cursor
// that hands out the ranges in the order that the read builds the lists and tuples, and the makers of a list
// value with a range.
func (t *TypedGen) rangeHelpers(b *strings.Builder) {
	if !t.anyRanges() {
		return
	}
	b.WriteString(`// zpRangeCursor hands out the ranges that the lists and tuples of a field had where they were made, in the
// order that a read of the field builds them: a list or a tuple before its elements, the elements in order, and
// only those that are not nil. A field whose lists and tuples hold no list or tuple stores the one range in
// One; a field of nested lists and tuples stores them all in All.
type zpRangeCursor struct {
	One rt.Span
	All []rt.Span
	I   int
}

// Next returns the next range; a cursor that has none left hands out the empty range at 0.
func (c *zpRangeCursor) Next() rt.Span {
	if c.All == nil {
		return c.One
	}
	if c.I >= len(c.All) {
		return rt.Span{}
	}
	r := c.All[c.I]
	c.I++
	return r
}

// zpListAt returns the list value of kids with the range r that the list had where it was made.
func zpListAt(kids []any, r rt.Span) any {
	return &rt.TSeq{Typ: rt.TypeListValue, Kids: kids, Span: r}
}

// zpListFieldAt is zpListAt that charges a step for each element (TParser.ListField).
func zpListFieldAt(p *rt.TParser, kids []any, r rt.Span) any {
	n := p.ListField(kids).(*rt.TSeq)
	n.Span = r
	return n
}

`)
}
