package core

import (
	"unicode/utf8"

	"zeroparse.dev/zp/runtime/parse"
)

// spendRead and Spend have a statically removable instrumentation boundary.
func (in *Input) spendRead(n uint64) {
	if BudgetControlsEnabled && in.Control != nil {
		in.Control.Charge(n)
	}
}

// spendReadDoc charges n units of reading that only a Document does (BudgetControl.ChargeDoc).
func (in *Input) spendReadDoc(n uint64) {
	if BudgetControlsEnabled && in.Control != nil && n > 0 {
		in.Control.ChargeDoc(n)
	}
}

func (p *Parser) Spend() {
	if BudgetControlsEnabled && p.Control != nil {
		p.Control.Charge(1)
	}
}

// SpendN charges n units as n calls of Spend do (BudgetControl.ChargeUnits), behind the same
// statically removable boundary.
func (p *Parser) SpendN(n uint64) {
	if BudgetControlsEnabled && p.Control != nil {
		p.Control.ChargeUnits(n)
	}
}

// NewControlledTextInput preserves the ordinary preparation path when inactive.
// Controlled decoding reserves up to four bytes before each UTF-8 decoder call;
// preparation counts both the sizing pass and the storage pass. Allocations
// themselves remain nonpreemptible, as do runtime buffer growth and GC.
func NewControlledTextInput(s string, unit parse.PositionUnit, offsets bool, sc *scratch, c *BudgetControl) Input {
	if !BudgetControlsEnabled || c == nil {
		return newTextInputIn(s, unit, offsets, sc)
	}
	if sc == nil {
		sc = &scratch{}
	}
	in := Input{Unit: unit, Eof: true, BaseLine: 1, BaseCol: 1, Control: c}
	if unit == parse.Bytes {
		in.Bs = grow(sc.bytes, len(s))
		for off := 0; off < len(s); {
			n := min(4096, len(s)-off)
			c.Charge(uint64(n))
			copy(in.Bs[off:off+n], s[off:off+n])
			off += n
		}
		in.Src, in.SrcOK = s, true
		return in
	}
	// Each of the two passes charges one unit per byte of the source, in chunks of one poll
	// interval, before the bytes of a chunk are decoded: cancellation is observed as often as
	// when each byte is charged alone.
	offsets = offsets && len(s) <= offsLimit
	n, valid := 0, true
	for off, charged := 0, 0; off < len(s); {
		if off >= charged {
			k := min(int(BudgetCheckInterval), len(s)-charged)
			c.Charge(uint64(k))
			charged += k
		}
		r, size := utf8.DecodeRuneInString(s[off:])
		if r == utf8.RuneError && size == 1 {
			valid = false
		}
		n++
		off += size
	}
	in.In = grow(sc.runes, n)
	if offsets {
		in.Offs = grow(sc.offs, n+1)
		in.Offs[n] = int32(len(s))
	}
	for off, i, charged := 0, 0, 0; off < len(s); i++ {
		if off >= charged {
			k := min(int(BudgetCheckInterval), len(s)-charged)
			c.Charge(uint64(k))
			charged += k
		}
		r, size := utf8.DecodeRuneInString(s[off:])
		in.In[i] = r
		if offsets {
			in.Offs[i] = int32(off)
		}
		off += size
	}
	in.Src, in.SrcOK = s, valid && len(s) <= offsLimit
	if !in.SrcOK {
		in.Offs = nil
	}
	in.setWideSource(s, valid)
	return in
}

// SpendDoc charges one unit of work that only a Document parse does (BudgetControl.ChargeDoc).
func (p *Parser) SpendDoc() {
	if BudgetControlsEnabled && p.Control != nil {
		p.Control.ChargeDoc(1)
	}
}

// FreeBegin opens a region of Document work (BudgetControl.FreeBegin) if on holds, and returns its mark.
func (p *Parser) FreeBegin(on bool) uint64 {
	if on && BudgetControlsEnabled && p.Control != nil {
		return p.Control.FreeBegin()
	}
	return noFree
}

// FreeEnd ends the region that FreeBegin opened.
func (p *Parser) FreeEnd(mark uint64) {
	if mark != noFree && BudgetControlsEnabled && p.Control != nil {
		p.Control.FreeEnd(mark)
	}
}
