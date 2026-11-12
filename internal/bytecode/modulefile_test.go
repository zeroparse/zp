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
