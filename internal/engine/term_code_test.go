package engine

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"reflect"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/bytecode"
	"zeroparse.dev/zp/internal/engine/vm"
	"zeroparse.dev/zp/internal/gentest"
)

// cloneTerms returns a copy of the term code ts that shares no slice with it.
func cloneTerms(ts []bytecode.Term) []bytecode.Term {
	out := make([]bytecode.Term, len(ts))
	for i, t := range ts {
		out[i] = bytecode.Term{
			Regs:      t.Regs,
			Code:      append([]bytecode.TermInstr(nil), t.Code...),
			Consts:    append([]bytecode.TermConst(nil), t.Consts...),
			Names:     append([]int(nil), t.Names...),
			Members:   append([]bytecode.TermMember(nil), t.Members...),
			NodeTypes: append([]int(nil), t.NodeTypes...),
			Casts:     append([]bytecode.TermCast(nil), t.Casts...),
			Structs:   append([]bytecode.TermStruct(nil), t.Structs...),
		}
	}
	return out
}

// TestTermCodeRoundTrip checks, for every grammar of the corpus and of testdata/grammars, that the
// term code of its module survives a module file: the term code that a loaded program holds equals
// the compiled program's, and loading the term code and exporting it again gives it back. A
// corpus with no term code would make the test vacuous, so it requires terms of every pool.
func TestTermCodeRoundTrip(t *testing.T) {
	t.Parallel()
	cases := gentest.CorpusWithParsers(t)
	type result struct {
		terms int
		pools [6]bool
		err   string
	}
	results := mapParallel(len(cases), func(i int) (r result) {
		prog, ok := coreCorpusProgram(t, cases[i])
		if !ok {
			return
		}
		m := prog.Module()
		data, err := prog.MarshalBinary("")
		if err != nil {
			r.err = err.Error()
			return
		}
		loaded, _, err := LoadProgram(data)
		if err != nil {
			r.err = err.Error()
			return
		}
		lm := loaded.Module()
		if !reflect.DeepEqual(normalTerms(m.Terms), normalTerms(lm.Terms)) {
			r.err = cases[i].Name + ": loaded term code differs"
			return
		}
		progs, err := vm.LoadTerms(m)
		if err != nil {
			r.err = err.Error()
			return
		}
		for k, tp := range progs {
			strs := map[string]int{}
			for i, s := range m.Strings {
				if _, ok := strs[s]; !ok {
					strs[s] = i
				}
			}
			x, err := tp.Export(func(s string) int { return strs[s] })
			if err != nil {
				r.err = err.Error()
				return
			}
			if !reflect.DeepEqual(normalTerm(x), normalTerm(canonicalStrings(m, m.Terms[k]))) {
				r.err = cases[i].Name + ": exported term code differs"
				return
			}
			t := &m.Terms[k]
			for j, n := range []int{len(t.Consts), len(t.Names), len(t.Members), len(t.NodeTypes), len(t.Casts), len(t.Structs)} {
				r.pools[j] = r.pools[j] || n > 0
			}
		}
		r.terms = len(m.Terms)
		return
	})
	terms := 0
	var pools [6]bool
	for _, r := range results {
		if r.err != "" {
			t.Error(r.err)
		}
		terms += r.terms
		for j := range pools {
			pools[j] = pools[j] || r.pools[j]
		}
	}
	if terms < 100 {
		t.Fatalf("%d terms in the corpus", terms)
	}
	for j, ok := range pools {
		if !ok {
			t.Errorf("no term of the corpus has pool %d", j)
		}
	}
}

// canonicalStrings returns t with every string index replaced by the first index of the same
// string, as an export with a first-index string table makes it.
func canonicalStrings(m *bytecode.Module, t bytecode.Term) bytecode.Term {
	first := map[string]int{}
	for i, s := range m.Strings {
		if _, ok := first[s]; !ok {
			first[s] = i
		}
	}
	c := func(i int) int {
		if i < 0 {
			return i
		}
		return first[m.Strings[i]]
	}
	t = cloneTerms([]bytecode.Term{t})[0]
	for i := range t.Consts {
		if t.Consts[i].Kind == bytecode.ConstString {
			t.Consts[i].Int = int64(c(int(t.Consts[i].Int)))
		}
	}
	for i := range t.Names {
		t.Names[i] = c(t.Names[i])
	}
	for i := range t.Members {
		t.Members[i].Name, t.Members[i].Field = c(t.Members[i].Name), c(t.Members[i].Field)
	}
	for i := range t.NodeTypes {
		t.NodeTypes[i] = c(t.NodeTypes[i])
	}
	for i := range t.Casts {
		t.Casts[i].Type = c(t.Casts[i].Type)
	}
	for i := range t.Structs {
		t.Structs[i].Type = c(t.Structs[i].Type)
	}
	return t
}

// normalTerm returns t with empty pools as nil, which a decoded module may hold as empty slices.
func normalTerm(t bytecode.Term) bytecode.Term {
	if len(t.Code) == 0 {
		t.Code = nil
	}
	if len(t.Consts) == 0 {
		t.Consts = nil
	}
	if len(t.Names) == 0 {
		t.Names = nil
	}
	if len(t.Members) == 0 {
		t.Members = nil
	}
	if len(t.NodeTypes) == 0 {
		t.NodeTypes = nil
	}
	if len(t.Casts) == 0 {
		t.Casts = nil
	}
	if len(t.Structs) == 0 {
		t.Structs = nil
	}
	return t
}

func normalTerms(ts []bytecode.Term) []bytecode.Term {
	out := make([]bytecode.Term, len(ts))
	for i, t := range ts {
		out[i] = normalTerm(t)
	}
	return out
}

// termCodeSource has terms with every pool: constants, names, members, conversions, casts and
// struct literals, a variable and a predicate.
const termCodeSource = `type S = { A node[string]; B int }
type N = node[string]
var n int = 1
rule main S = a:"a" &{ len(text($a)) > 0 } b:"b" => S{A: N{$a}, B: len(map(list($b.Start, n), (x) => x.(int) + 1))}`

// TestTermCodeValidation checks that a module file whose term code is invalid is rejected, for each
// check of the loader: the pools refer to strings, conversions to named node types, struct literals
// to struct types and casts to valid types; the instructions are known, their operands are in
// range for their kinds, and the code ends with a return or a jump. The unchanged file loads, the
// negative control.
func TestTermCodeValidation(t *testing.T) {
	t.Parallel()
	prog := mustCompile(t, termCodeSource)
	base := prog.Module()
	data, err := prog.MarshalBinary("main")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := LoadProgram(data); err != nil {
		t.Fatalf("unchanged module: %v", err)
	}
	// find returns the first term with a nonempty pool of the kind.
	find := func(m *bytecode.Module, has func(*bytecode.Term) bool) *bytecode.Term {
		for i := range m.Terms {
			if has(&m.Terms[i]) {
				return &m.Terms[i]
			}
		}
		t.Fatal("no term with the pool")
		return nil
	}
	str := func(m *bytecode.Module, s string) int {
		for i, x := range m.Strings {
			if x == s {
				return i
			}
		}
		t.Fatalf("no string %q", s)
		return -1
	}
	for _, tc := range []struct {
		name string
		edit func(m *bytecode.Module)
		want string
	}{
		{"name string", func(m *bytecode.Module) {
			find(m, func(x *bytecode.Term) bool { return len(x.Names) > 0 }).Names[0] = len(m.Strings)
		}, "string"},
		{"const string", func(m *bytecode.Module) {
			x := find(m, func(x *bytecode.Term) bool { return len(x.Consts) > 0 })
			x.Consts[0] = bytecode.TermConst{Kind: bytecode.ConstString, Int: int64(len(m.Strings))}
		}, "string"},
		{"const kind", func(m *bytecode.Module) {
			find(m, func(x *bytecode.Term) bool { return len(x.Consts) > 0 }).Consts[0].Kind = 9
		}, "invalid constant kind"},
		{"member string", func(m *bytecode.Module) {
			find(m, func(x *bytecode.Term) bool { return len(x.Members) > 0 }).Members[0].Field = -2
		}, "string"},
		{"member kind", func(m *bytecode.Module) {
			find(m, func(x *bytecode.Term) bool { return len(x.Members) > 0 }).Members[0].Kind = 99
		}, "invalid member kind"},
		{"node type", func(m *bytecode.Module) {
			find(m, func(x *bytecode.Term) bool { return len(x.NodeTypes) > 0 }).NodeTypes[0] = str(m, "S")
		}, "is not a named node type"},
		{"struct type", func(m *bytecode.Module) {
			find(m, func(x *bytecode.Term) bool { return len(x.Structs) > 0 }).Structs[0].Type = str(m, "N")
		}, "is not a struct type"},
		{"cast type", func(m *bytecode.Module) {
			x := find(m, func(x *bytecode.Term) bool { return len(x.Casts) > 0 })
			m.Strings = append(m.Strings, "Other")
			x.Casts[0].Type = len(m.Strings) - 1
		}, "invalid field type"},
		{"cast position", func(m *bytecode.Module) {
			find(m, func(x *bytecode.Term) bool { return len(x.Casts) > 0 }).Casts[0].Line = -1
		}, "invalid cast position"},
		{"opcode", func(m *bytecode.Module) {
			m.Terms[0].Code[0].Op = 250
		}, "invalid term instruction"},
		{"register", func(m *bytecode.Module) {
			m.Terms[0].Regs = [4]int{}
		}, "invalid operand"},
		{"jump", func(m *bytecode.Module) {
			x := find(m, func(x *bytecode.Term) bool {
				for _, in := range x.Code {
					if in.Op == m.Terms[0].Code[len(m.Terms[0].Code)-1].Op {
						return true
					}
				}
				return false
			})
			x.Code[len(x.Code)-1] = bytecode.TermInstr{Op: jumpOp(t, m), Dst: int32(len(x.Code))}
		}, "invalid operand"},
		{"end", func(m *bytecode.Module) {
			x := &m.Terms[0]
			x.Code = append(x.Code, x.Code[0])
			if x.Code[0].Op == x.Code[len(x.Code)-2].Op {
				t.Fatal("first instruction is a return")
			}
		}, "does not end"},
		{"empty", func(m *bytecode.Module) {
			m.Terms[0].Code = nil
		}, "empty term code"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := *base
			m.Strings = append([]string(nil), base.Strings...)
			m.Terms = cloneTerms(base.Terms)
			tc.edit(&m)
			w := &bytecode.Encoder{Strs: map[string]int{}}
			for i, s := range m.Strings {
				w.Strs[s] = i
				w.List = append(w.List, s)
			}
			w.Str("main")
			w.WriteModule(&m)
			file := recompose(t, data[:isaOffset+1], w)
			if _, _, err := LoadProgram(file); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want %q", err, tc.want)
			}
		})
	}
}

// jumpOp returns the opcode of an unconditional jump of term code: that of the instruction of the
// module's term code whose only operand is a jump target, found by loading variants.
func jumpOp(t *testing.T, m *bytecode.Module) uint8 {
	t.Helper()
	for op := 0; op < 256; op++ {
		x := bytecode.Term{Regs: [4]int{1, 1, 1, 1}, Code: []bytecode.TermInstr{{Op: uint8(op), Dst: 0}}}
		y := bytecode.Term{Regs: [4]int{1, 1, 1, 1}, Code: []bytecode.TermInstr{{Op: uint8(op), Dst: 1}}}
		mx, my := *m, *m
		mx.Terms, my.Terms = []bytecode.Term{x}, []bytecode.Term{y}
		if _, err := vm.LoadTerms(&mx); err != nil {
			continue
		}
		if _, err := vm.LoadTerms(&my); err != nil {
			return uint8(op) // a jump to 0 loads and ends the code; a jump past it does not
		}
	}
	t.Fatal("no jump instruction")
	return 0
}

// recompose returns the module file whose header (magic, version, mode and instruction-set version)
// is head and whose payload w encodes, with its checksum.
func recompose(t *testing.T, head []byte, w *bytecode.Encoder) []byte {
	t.Helper()
	var out bytes.Buffer
	out.Write(head)
	w.Finish(&out)
	b := out.Bytes()
	if crc32.ChecksumIEEE(b[:len(b)-4]) != binary.LittleEndian.Uint32(b[len(b)-4:]) {
		t.Fatal("checksum")
	}
	return b
}
