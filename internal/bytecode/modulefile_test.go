package bytecode_test

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/bytecode"
)

// TestTermCodeEncoding checks that term code survives writing and reading, with operands that are
// zero, negative and large, and that reading rejects an instruction whose operand mask has bits
// beyond the four operands or marks an operand that is zero, so that each instruction has one
// encoding.
func TestTermCodeEncoding(t *testing.T) {
	code := []bytecode.TermInstr{
		{Op: 3, Dst: 1, A: 0, B: 2},
		{Op: 41, A: -1, C: 1 << 20},
		{Op: 46},
		{Op: 7, Dst: 5},
	}
	m := &bytecode.Module{Terms: []bytecode.Term{{Regs: [4]int{6, 1, 0, 0}, Code: code}}}
	w := &bytecode.Encoder{Strs: map[string]int{}}
	w.WriteModule(m)
	data := w.Body.Bytes()
	r := &bytecode.Decoder{Data: data}
	got := r.ReadModule()
	if r.Err != nil {
		t.Fatal(r.Err)
	}
	if r.Pos != len(data) {
		t.Fatalf("read %d of %d bytes", r.Pos, len(data))
	}
	if !reflect.DeepEqual(got.Terms[0].Code, code) {
		t.Fatalf("code read back as %v, want %v", got.Terms[0].Code, code)
	}
	// The last instruction is written last: opcode 7, mask 1 (D only) and D = 5 (zigzag 10).
	if !bytes.HasSuffix(data, []byte{7, 1, 10}) {
		t.Fatalf("the module does not end with the last instruction: % x", data[len(data)-3:])
	}
	for _, c := range []struct {
		name string
		tail []byte
		want string
	}{
		{"mask beyond the operands", []byte{7, 0x11, 10}, "invalid term operand mask"},
		{"marked zero operand", []byte{7, 1, 0}, "term operand marked but zero"},
	} {
		bad := append(bytes.Clone(data[:len(data)-3]), c.tail...)
		r := &bytecode.Decoder{Data: bad}
		r.ReadModule()
		if r.Err == nil || !strings.Contains(r.Err.Error(), c.want) {
			t.Errorf("%s: error %v, want %q", c.name, r.Err, c.want)
		}
	}
}

// TestTermPoolEncoding checks that the pools of a term survive writing and reading whichever of
// them are empty, and that reading rejects a pool mask with bits beyond the six pools or a marked
// pool that is empty, so that each term has one encoding.
func TestTermPoolEncoding(t *testing.T) {
	ret := []bytecode.TermInstr{{Op: 46}}
	full := bytecode.Term{Regs: [4]int{1, 2, 3, 4}, Code: ret,
		Consts:    []bytecode.TermConst{{Kind: bytecode.ConstInt, Int: -7}},
		Names:     []int{3, 0},
		Members:   []bytecode.TermMember{{Kind: 1, Name: 2, Field: 3}},
		NodeTypes: []int{5},
		Casts:     []bytecode.TermCast{{Type: -1}},
		Structs:   []bytecode.TermStruct{{Type: 4, Fields: 2}}}
	some := bytecode.Term{Regs: [4]int{1}, Code: ret, Names: []int{1}, Structs: []bytecode.TermStruct{{Type: 2, Fields: 1}}}
	none := bytecode.Term{Regs: [4]int{1}, Code: ret}
	m := &bytecode.Module{Terms: []bytecode.Term{full, some, none}}
	w := &bytecode.Encoder{Strs: map[string]int{}}
	w.WriteModule(m)
	data := w.Body.Bytes()
	r := &bytecode.Decoder{Data: data}
	got := r.ReadModule()
	if r.Err != nil {
		t.Fatal(r.Err)
	}
	if !reflect.DeepEqual(got.Terms, m.Terms) {
		t.Fatalf("terms read back as %+v, want %+v", got.Terms, m.Terms)
	}
	// The last term is written last: its pool mask 0, one instruction, RET with no operands.
	if !bytes.HasSuffix(data, []byte{0, 1, 46, 0}) {
		t.Fatalf("the module does not end with the last term: % x", data[len(data)-4:])
	}
	for _, c := range []struct {
		name string
		tail []byte
		want string
	}{
		{"mask beyond the pools", []byte{0x40, 1, 46, 0}, "invalid term pool mask"},
		{"marked empty pool", []byte{0x01, 0, 1, 46, 0}, "term pool marked but empty"},
	} {
		bad := append(bytes.Clone(data[:len(data)-4]), c.tail...)
		r := &bytecode.Decoder{Data: bad}
		r.ReadModule()
		if r.Err == nil || !strings.Contains(r.Err.Error(), c.want) {
			t.Errorf("%s: error %v, want %q", c.name, r.Err, c.want)
		}
	}
}
