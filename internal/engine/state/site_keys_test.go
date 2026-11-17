package state

import "testing"

// TestSiteKeyNumbersValues checks the key of the records of a run site that is not free of the
// environment: environments that were built apart but hold equal values of the variables that the
// element reads have one number, whatever the values of the other variables, and environments that
// differ in one of those variables have two. Keys by the environment as an object, which the
// mutation SkipKeyByIdentity restores, give built-apart environments two numbers.
func TestSiteKeyNumbersValues(t *testing.T) {
	defer func() { SkipRecordMutation = 0 }()
	key := NewSiteKey([]int{0, 2}, nil)
	for _, mutation := range []int{0, SkipKeyByIdentity} {
		SkipRecordMutation = mutation
		p := &Parser{}
		p.SetVars([]any{1, "a", true})
		first := p.siteKey(key)
		p.Bind(0, 2)
		p.Bind(0, 1) // equal values, another object
		p.Bind(1, "b")
		same := p.siteKey(key)
		p.Bind(2, false)
		other := p.siteKey(key)
		if mutation == 0 {
			if first != same {
				t.Errorf("equal values of the variables read have numbers %d and %d", first, same)
			}
			if other == same {
				t.Errorf("another value of a variable read has the number %d of the first", other)
			}
		} else if first == same {
			t.Errorf("mutation %d: environments built apart have one number", mutation)
		}
	}
}

// TestSiteKeyListsVariables checks the variables of a key: those read and bound once each, the bound
// ones that are not read in Write, and slots of undeclared variables left out.
func TestSiteKeyListsVariables(t *testing.T) {
	k := NewSiteKey([]int{2, 0, 2, -1}, []int{1, 2, 1, -1, 3})
	if want := []int{2, 0}; !equalInts(k.Read, want) {
		t.Errorf("Read %v, want %v", k.Read, want)
	}
	if want := []int{1, 2, 3}; !equalInts(k.Bind, want) {
		t.Errorf("Bind %v, want %v", k.Bind, want)
	}
	if want := []int{1, 3}; !equalInts(k.Write, want) || !equalInts(k.write, []int{0, 2}) {
		t.Errorf("Write %v at %v, want %v at [0 2]", k.Write, k.write, want)
	}
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// keyedWalk runs a walk of site 1 from pos with key, whose iterations match one position each until
// the one at end, which fails; the iteration at bindAt binds the variable in slot 1 to 9, and every
// iteration reads the variable in slot 0 (the key holds it).
func keyedWalk(p *Parser, key *SiteKey, pos, end, bindAt int) {
	p.Pos = pos
	p.SkipBegin(1, key)
	for {
		if p.SkipHit() {
			break
		}
		p.SkipVisit()
		if p.Pos == end {
			break
		}
		if p.Pos == bindAt {
			p.Bind(1, 9)
		}
		p.Pos++
	}
	p.SkipEnd()
}

// TestRunSiteRecordsBindOnlyWhatTheyBound checks the records of a site whose element binds a
// variable that it does not read: a walk that takes a record gets the value that the rest of the
// recorded walk bound the variable to, if it bound it, and otherwise keeps its own, however the
// variable was set before the walk; a variable that the element reads keys the records. Without the
// hiding of the variable while a walk records (SkipNoHide), a walk that takes a record keeps its
// own value where the recorded walk bound one.
func TestRunSiteRecordsBindOnlyWhatTheyBound(t *testing.T) {
	defer func() { SkipRecordMutation = 0 }()
	key := NewSiteKey([]int{0}, []int{1})
	for _, mutation := range []int{0, SkipNoHide} {
		SkipRecordMutation = mutation
		p := &Parser{}
		p.SetVars([]any{0, 0})
		keyedWalk(p, key, 0, 6, 2) // plain: marks
		keyedWalk(p, key, 0, 6, 2) // records every position
		if got := p.Env.Get(1, 0); got != 9 {
			t.Fatalf("mutation %d: the walk that binds leaves %v", mutation, got)
		}
		wrong := false
		for _, c := range []struct{ start, before, want int }{
			{1, 3, 9}, // the rest from 1 binds at 2
			{3, 3, 3}, // the rest from 3 binds nothing
			{2, 5, 9},
			{0, 4, 9},
			{4, 4, 4},
		} {
			p.Bind(1, c.before)
			keyedWalk(p, key, c.start, 6, -1) // (a walk that evaluated would not bind: a hit must)
			if got := p.Env.Get(1, 0); got != c.want {
				if mutation == 0 {
					t.Errorf("a walk from %d with %d leaves %v, want %d", c.start, c.before, got, c.want)
				}
				wrong = true
			}
			if p.Pos != 6 {
				t.Errorf("mutation %d: a walk from %d ends at %d", mutation, c.start, p.Pos)
			}
		}
		if mutation != 0 && !wrong {
			t.Errorf("mutation %d: every walk leaves the right value", mutation)
		}
		// Another value of the variable read is another key: the walk evaluates its iterations,
		// which here bind nothing.
		p.Bind(0, 1)
		p.Bind(1, 5)
		keyedWalk(p, key, 1, 6, -1)
		if got := p.Env.Get(1, 0); got != 5 {
			t.Errorf("mutation %d: a walk under another key leaves %v, want 5", mutation, got)
		}
	}
}
