package lint

import (
	"reflect"
	"testing"

	"zeroparse.dev/zp/internal/analysis/complexity"
)

// TestClassifyMirrorsImplementation checks that the classes and cause kinds of the public package
// have the values and names of the classifier's, which Classify converts by value, and that Cause
// keeps the fields of the classifier's cause, so that a class or kind added to one side cannot be
// misread on the other.
func TestClassifyMirrorsImplementation(t *testing.T) {
	for _, p := range []struct {
		pub Class
		in  complexity.ComplexityClass
	}{
		{Linear, complexity.ClassLinear}, {LinearEnv, complexity.ClassLinearEnv},
		{Polynomial, complexity.ClassPoly}, {Unknown, complexity.ClassUnknown},
		{Exponential, complexity.ClassExp}, {99, 99},
	} {
		if int(p.pub) != int(p.in) || p.pub.String() != p.in.String() {
			t.Errorf("class %d is %q, the classifier has %d as %q", int(p.pub), p.pub, int(p.in), p.in)
		}
	}
	// The classifier has no class after ClassExp.
	if s := complexity.ComplexityClass(int(Exponential) + 1).String(); s != "ComplexityClass(5)" {
		t.Errorf("the classifier names class %d %q; the public package has no constant for it", int(Exponential)+1, s)
	}
	for _, p := range []struct {
		pub CauseKind
		in  complexity.ComplexityCauseKind
	}{
		{CauseOther, complexity.CauseOther}, {CauseRestartedRepetition, complexity.CauseRestartedRepetition},
		{CauseLookaheadScan, complexity.CauseLookaheadScan}, {CauseRecoverySkip, complexity.CauseRecoverySkip},
		{CauseRegrowth, complexity.CauseRegrowth}, {CauseDivergingBinding, complexity.CauseDivergingBinding},
		{CauseSelfGrowingValue, complexity.CauseSelfGrowingValue}, {99, 99},
	} {
		if int(p.pub) != int(p.in) || p.pub.String() != p.in.String() {
			t.Errorf("cause kind %d is %q, the classifier has %d as %q", int(p.pub), p.pub, int(p.in), p.in)
		}
	}
	if s := complexity.ComplexityCauseKind(int(CauseSelfGrowingValue) + 1).String(); s != "ComplexityCauseKind(7)" {
		t.Errorf("the classifier names cause kind %d %q; the public package has no constant for it", int(CauseSelfGrowingValue)+1, s)
	}
	pub, in := reflect.TypeOf(Cause{}), reflect.TypeOf(complexity.ComplexityCause{})
	if pub.NumField() != in.NumField() {
		t.Fatalf("Cause has %d fields, the classifier's cause has %d", pub.NumField(), in.NumField())
	}
	for i := 0; i < pub.NumField(); i++ {
		if pub.Field(i).Name != in.Field(i).Name {
			t.Errorf("Cause field %d is %s, the classifier has %s", i, pub.Field(i).Name, in.Field(i).Name)
		}
	}
}
