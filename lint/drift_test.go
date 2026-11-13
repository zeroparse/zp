package lint

import (
	"reflect"
	"testing"

	impl "zeroparse.dev/zp/internal/lint"
)

// TestMirrorsImplementation checks that the public types, which are copies of the types of the
// implementation, keep the same fields (names, order and types, with Severity standing for the
// implementation's Severity) and the same severity values and names, and that the constants and
// Checks agree, so that a change to one copy cannot go unnoticed in the other.
func TestMirrorsImplementation(t *testing.T) {
	sev := reflect.TypeOf(Severity(0))
	implSev := reflect.TypeOf(impl.Severity(0))
	for _, p := range []struct{ pub, in reflect.Type }{
		{reflect.TypeOf(Finding{}), reflect.TypeOf(impl.Finding{})},
		{reflect.TypeOf(Check{}), reflect.TypeOf(impl.Check{})},
		{reflect.TypeOf(Config{}), reflect.TypeOf(impl.Config{})},
		{reflect.TypeOf(Rewrite{}), reflect.TypeOf(impl.Rewrite{})},
	} {
		if p.pub.NumField() != p.in.NumField() {
			t.Errorf("%s has %d fields, the implementation has %d", p.pub.Name(), p.pub.NumField(), p.in.NumField())
			continue
		}
		for i := 0; i < p.pub.NumField(); i++ {
			a, b := p.pub.Field(i), p.in.Field(i)
			bt := b.Type
			if bt == implSev {
				bt = sev
			}
			if a.Name != b.Name || a.Type != bt || a.Tag != b.Tag {
				t.Errorf("%s field %d is %s %v, the implementation has %s %v", p.pub.Name(), i, a.Name, a.Type, b.Name, b.Type)
			}
		}
	}
	if sev.Kind() != implSev.Kind() {
		t.Errorf("Severity is %v, the implementation has %v", sev.Kind(), implSev.Kind())
	}
	for _, p := range []struct {
		pub Severity
		in  impl.Severity
	}{{Hint, impl.Hint}, {Warning, impl.Warning}, {Error, impl.Error}, {0, 0}, {99, 99}} {
		if int(p.pub) != int(p.in) || p.pub.String() != p.in.String() {
			t.Errorf("severity %d is %q, the implementation has %d as %q", int(p.pub), p.pub, int(p.in), p.in)
		}
	}
	names := map[string]string{
		CheckUnreachableRule: impl.CheckUnreachableRule, CheckShadowedAlt: impl.CheckShadowedAlt,
		CheckNeverMatches: impl.CheckNeverMatches, CheckUselessLookahead: impl.CheckUselessLookahead,
		CheckNullableRepetition: impl.CheckNullableRepetition, CheckRedundantOptional: impl.CheckRedundantOptional,
		CheckGrowingVariable: impl.CheckGrowingVariable, CheckUnusedCapture: impl.CheckUnusedCapture,
		CheckDuplicateCapture: impl.CheckDuplicateCapture, CheckCharClass: impl.CheckCharClass,
		CheckRightRecursion: impl.CheckRightRecursion, CheckPositions: impl.CheckPositions,
		CheckLongLookahead: impl.CheckLongLookahead, CheckSuperLinear: impl.CheckSuperLinear,
		CheckCaseInsensitive: impl.CheckCaseInsensitive, CheckDirective: impl.CheckDirective,
	}
	if len(names) != 16 {
		t.Errorf("the check-name constants are %d distinct values, want 16", len(names))
	}
	for pub, in := range names {
		if pub != in {
			t.Errorf("check name %q, the implementation has %q", pub, in)
		}
	}
	want := impl.Checks()
	got := Checks()
	if len(got) != len(want) {
		t.Fatalf("Checks has %d entries, the implementation has %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Name != want[i].Name || int(got[i].Severity) != int(want[i].Severity) || got[i].Summary != want[i].Summary {
			t.Errorf("Checks()[%d] = %+v, the implementation has %+v", i, got[i], want[i])
		}
	}
}
