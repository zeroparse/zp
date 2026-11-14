package libgo

import (
	"math/rand"
	"slices"
	"testing"

	"zeroparse.dev/zp/core"
)

// The hand-written forms of the expectation record that the compositions of internal/ir/exec/core
// replaced, kept as the reference model of TestExpectRecordMatchesReference.

// refExpect records no examined input, in a Document parse too: the read that failed at pos did.
func (p *Parser) refExpect(pos int, id ExpID) {
	if p.Silent > 0 || pos < p.farthest {
		return
	}
	if pos > p.farthest {
		p.farthest = pos
		p.exp = p.exp[:p.expBase]
		p.expBits = 0
	}
	if !p.Expectations {
		if len(p.exp) == p.expBase {
			p.exp = append(p.exp, ExpFar)
		}
		return
	}
	bit := uint64(1) << (uint32(id) * 0x9e3779b1 >> 26)
	if p.expBits&bit != 0 {
		for _, e := range p.exp[p.expBase:] {
			p.Spend()
			if e == id {
				return
			}
		}
	}
	p.expBits |= bit
	p.exp = append(p.exp, id)
}

func (p *Parser) refExpectOrdinary(pos int, id ExpID) {
	if p.Silent > 0 || pos < p.farthest {
		return
	}
	p.refExpectOrdinarySlow(pos, id)
}

func (p *Parser) refExpectOrdinarySlow(pos int, id ExpID) {
	if !p.Expectations {
		id = ExpFar
	}
	if pos > p.farthest {
		p.farthest = pos
		p.exp = p.exp[:p.expBase]
		p.expBits = 0
	}
	bit := uint64(1) << (uint32(id) * 0x9e3779b1 >> 26)
	if p.expBits&bit != 0 {
		for _, e := range p.exp[p.expBase:] {
			if e == id {
				return
			}
		}
	}
	p.expBits |= bit
	p.exp = append(p.exp, id)
}

// refMergeExpected is MergeExpected over refExpect.
func (p *Parser) refMergeExpected(far int, ids []ExpID) {
	for _, x := range ids {
		p.Spend()
		p.refExpect(far, x)
	}
}

// refKeep is the hand-written Keep that the composition keep replaced.
func (p *Parser) refKeep(ids []ExpID) []ExpID {
	if len(ids) == 0 {
		return nil
	}
	if p.refEqualExpected(ids, p.lastKept) { // the same set often repeats
		return p.lastKept
	}
	if cap(p.arena)-len(p.arena) < len(ids) {
		// A new chunk; the previous one lives while slices into it remain.
		p.arena = make([]ExpID, 0, max(1024, len(ids)))
		p.memUsed += uint64(cap(p.arena)) * expIDBytes
	}
	off := len(p.arena)
	p.arena = appendSpend(p, p.arena, ids)
	p.lastKept = p.arena[off:len(p.arena):len(p.arena)]
	return p.lastKept
}

// refEqualExpected is the comparison of refKeep: it charges each compared entry in a parse with
// budget controls.
func (p *Parser) refEqualExpected(a, b []ExpID) bool {
	if p.Control == nil {
		return slices.Equal(a, b)
	}
	if len(a) != len(b) {
		return false
	}
	for i, x := range a {
		p.Spend()
		if x != b[i] {
			return false
		}
	}
	return true
}

// TestKeepMatchesReference keeps random sets through the printed composition and through the
// hand-written form it replaced, with and without budget controls, and checks after each call the
// set returned, whether it is the set kept last, the arena (its length and the chunk it fills), the
// memory accounted for its chunks and the units charged. The sets repeat often and some are longer
// than a chunk of the arena and than a chunk of a charged copy.
func TestKeepMatchesReference(t *testing.T) {
	for _, controlled := range []bool{false, true} {
		rng := rand.New(rand.NewSource(3))
		for round := 0; round < 200; round++ {
			newP := func() *Parser {
				p := &Parser{}
				if controlled {
					p.Control = newBudgetControl(nil, core.Limits{Compute: 1 << 40})
				}
				return p
			}
			a, b := newP(), newP()
			var pool [][]ExpID
			for step := 0; step < 40; step++ {
				var ids []ExpID
				switch k := rng.Intn(10); {
				case k < 4 && len(pool) > 0:
					ids = slices.Clone(pool[rng.Intn(len(pool))])
				case k < 5 && len(pool) > 0:
					// A set that differs from a kept one in its last entry only.
					ids = slices.Clone(pool[rng.Intn(len(pool))])
					if len(ids) > 0 {
						ids[len(ids)-1]++
					}
				default:
					n := rng.Intn(6)
					if rng.Intn(8) == 0 {
						n = 100 + rng.Intn(1200)
					}
					ids = make([]ExpID, n)
					for i := range ids {
						ids[i] = ExpID(1 + rng.Intn(4))
					}
					pool = append(pool, ids)
				}
				ka, kb := a.Keep(slices.Clone(ids)), b.refKeep(slices.Clone(ids))
				lastA := len(ka) > 0 && len(a.lastKept) > 0 && &ka[0] == &a.lastKept[0]
				lastB := len(kb) > 0 && len(b.lastKept) > 0 && &kb[0] == &b.lastKept[0]
				same := slices.Equal(ka, kb) && (ka == nil) == (kb == nil) && lastA == lastB &&
					slices.Equal(a.lastKept, b.lastKept) && len(a.arena) == len(b.arena) && cap(a.arena) == cap(b.arena) &&
					a.memUsed == b.memUsed
				if controlled {
					same = same && a.Control.budgetUsed() == b.Control.budgetUsed()
				}
				if !same {
					t.Fatalf("controlled %v round %d step %d keeping %d entries: last %v/%v arena %d,%d/%d,%d memUsed %d/%d",
						controlled, round, step, len(ids), lastA, lastB, len(a.arena), cap(a.arena), len(b.arena), cap(b.arena), a.memUsed, b.memUsed)
				}
			}
		}
	}
}

// TestExpectRecordMatchesReference runs random sequences of the operations on an expectation record
// through the printed compositions and through the hand-written forms they replaced, in parses that
// record expectations and parses that do not, with and without budget controls, and checks that
// the record (the farthest position, the entries and, where it is consulted, the filter of the
// entries), the frontier a Document parse examined and the units charged are the same after every
// operation.
func TestExpectRecordMatchesReference(t *testing.T) {
	for _, recording := range []bool{false, true} {
		for _, controlled := range []bool{false, true} {
			for _, document := range []bool{false, true} {
				rng := rand.New(rand.NewSource(1))
				for round := 0; round < 400; round++ {
					newP := func() *Parser {
						p := &Parser{Expectations: recording}
						if document {
							p.Document = &documentState{}
						}
						if controlled {
							p.Control = newBudgetControl(nil, core.Limits{Compute: 1 << 40})
						}
						return p
					}
					a, b := newP(), newP()
					// In a parse that does not record, the hand-written Ordinary form added a second
					// placeholder to a record that another form had given one; the error of a parse leaves
					// the placeholder out, and the compositions keep one. A parse that does not record
					// therefore uses the forms of one family.
					ordinary := rng.Intn(2) == 0
					type frame struct {
						ma, mb ExpMark
					}
					var stack []frame
					check := func(step int, what string) {
						t.Helper()
						same := a.farthest == b.farthest && a.Silent == b.Silent && a.docHigh == b.docHigh
						if recording {
							same = same && a.expBase == b.expBase && slices.Equal(a.exp, b.exp) && a.expBits == b.expBits
						} else {
							// The current record has the same entries but for repeats of the placeholder.
							ra, rb := slices.Compact(slices.Clone(a.exp[a.expBase:])), slices.Compact(slices.Clone(b.exp[b.expBase:]))
							same = same && slices.Equal(ra, rb)
						}
						if controlled {
							same = same && a.Control.budgetUsed() == b.Control.budgetUsed()
						}
						if !same {
							t.Fatalf("recording %v controlled %v document %v round %d step %d after %s: far %d/%d base %d/%d exp %v/%v silent %d/%d docHigh %d/%d",
								recording, controlled, document, round, step, what, a.farthest, b.farthest, a.expBase, b.expBase, a.exp, b.exp, a.Silent, b.Silent, a.docHigh, b.docHigh)
						}
					}
					for step := 0; step < 60; step++ {
						pos := rng.Intn(6)
						id := ExpID(1 + rng.Intn(90))
						if rng.Intn(5) == 0 {
							id = ExpFar
						}
						switch op := rng.Intn(9); op {
						case 0, 1:
							if !recording && ordinary {
								continue
							}
							a.Expect(pos, id)
							b.refExpect(pos, id)
							check(step, "Expect")
						case 2:
							if !recording && ordinary {
								continue
							}
							// A second weight for the one form that parses with limit controls or a Document take.
							a.Expect(pos, id)
							b.refExpect(pos, id)
							check(step, "Expect")
						case 3, 4:
							if controlled || !recording && !ordinary {
								continue // an ordinary parse has no budget controls
							}
							a.ExpectOrdinary(pos, id)
							b.refExpectOrdinary(pos, id)
							check(step, "ExpectOrdinary")
						case 5:
							if len(stack) < 4 {
								stack = append(stack, frame{a.Isolate(pos), b.Isolate(pos)})
							}
							check(step, "Isolate")
						case 6:
							if len(stack) > 0 {
								f := stack[len(stack)-1]
								stack = stack[:len(stack)-1]
								fa, ia := a.Unisolate(f.ma)
								fb, ib := b.Unisolate(f.mb)
								norm := func(ids []ExpID) []ExpID {
									if recording {
										return ids
									}
									return slices.Compact(slices.Clone(ids))
								}
								if fa != fb || !slices.Equal(norm(ia), norm(ib)) {
									t.Fatalf("Unisolate differs: %d %v / %d %v", fa, ia, fb, ib)
								}
								ka, kb := a.Keep(ia), b.refKeep(ib)
								if !slices.Equal(norm(ka), norm(kb)) {
									t.Fatalf("Keep differs")
								}
								a.MergeExpected(fa, ia)
								b.refMergeExpected(fb, ib)
							}
							check(step, "Unisolate and MergeExpected")
						case 7:
							a.Silent = rng.Intn(2)
							b.Silent = a.Silent
							check(step, "Silent")
						case 8:
							ids := []ExpID{id, ExpID(1 + rng.Intn(90)), ExpFar}[:rng.Intn(4)]
							a.MergeExpected(pos, ids)
							b.refMergeExpected(pos, ids)
							check(step, "MergeExpected")
						}
					}
				}
			}
		}
	}
}

// TestNonRecordingRecordHoldsOnePlaceholder mixes every form of the record in parses that do not
// record expectations, with isolations and merges, and checks the invariant that the current record
// holds at most one placeholder and no other entry. The error of such a parse needs only the
// farthest position.
func TestNonRecordingRecordHoldsOnePlaceholder(t *testing.T) {
	for _, controlled := range []bool{false, true} {
		rng := rand.New(rand.NewSource(2))
		for round := 0; round < 400; round++ {
			p := &Parser{}
			if controlled {
				p.Control = newBudgetControl(nil, core.Limits{Compute: 1 << 40})
			}
			var marks []ExpMark
			for step := 0; step < 80; step++ {
				pos := rng.Intn(6)
				id := ExpID(1 + rng.Intn(90))
				switch rng.Intn(8) {
				case 0:
					p.Expect(pos, id)
				case 1:
					p.Expect(pos, id)
				case 2:
					if !controlled {
						p.ExpectOrdinary(pos, id)
					}
				case 3:
					p.MergeExpected(pos, []ExpID{id, ExpFar})
				case 4:
					if len(marks) < 4 {
						marks = append(marks, p.Isolate(pos))
					}
				case 5:
					if len(marks) > 0 {
						far, ids := p.Unisolate(marks[len(marks)-1])
						marks = marks[:len(marks)-1]
						p.MergeExpected(far, ids)
					}
				case 6:
					p.Silent = rng.Intn(2)
				}
				cur := p.exp[p.expBase:]
				if len(cur) > 1 || len(cur) == 1 && cur[0] != ExpFar {
					t.Fatalf("controlled %v round %d step %d: the record holds %v", controlled, round, step, cur)
				}
			}
		}
	}
}
