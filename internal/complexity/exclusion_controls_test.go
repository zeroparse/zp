package complexity_test

import (
	"testing"
	"zeroparse.dev/zp/internal/complexity"

	"zeroparse.dev/zp/engine"
	"zeroparse.dev/zp/internal/complexitytest"
	"zeroparse.dev/zp/internal/starttest"
)

// TestExclusionControls classifies the complexity controls with the defaults of the classifier
// and with each setting of the hooks of the exclusion analysis (the zero setting is not listed:
// it is the classifier without the analysis, which the complexity tests cover). A control that is not positive grows faster than linearly in a measurement, so no
// setting that describes the parser as it is may classify it linear, and a positive control stays
// linear. Memoizing the trivia rules describes a parser that the backends do not build yet, so
// with it the controls that become linear are listed instead of failing.
func TestExclusionControls(t *testing.T) {
	settings := []struct {
		name     string
		h        complexity.ExclusionHooks
		defaults bool // no hooks set: the classifier as it is
	}{
		{name: "defaults", defaults: true},
		{name: "decided units", h: complexity.ExclusionHooks{Decided: true}},
		{name: "decided units, trivia", h: complexity.ExclusionHooks{Decided: true, Trivia: true}},
		{name: "memoized trivia", h: complexity.ExclusionHooks{Memo: true}},
		{name: "decided units, memoized trivia", h: complexity.ExclusionHooks{Decided: true, Trivia: true, Memo: true}},
	}
	for _, c := range complexitytest.Controls() {
		g, err := engine.ParseGrammar(c.Source)
		if err != nil {
			t.Fatalf("%s: %v", c.Name, err)
		}
		for _, s := range settings {
			restore := func() {}
			if !s.defaults {
				restore = s.h.Set()
			}
			got, err := complexity.ClassifyComplexityControlled(starttest.MarkAll(g), "main", nil)
			restore()
			if err != nil {
				t.Fatalf("%s: %s: %v", c.Name, s.name, err)
			}
			linear := got.Class == complexity.ClassLinear || got.Class == complexity.ClassLinearEnv
			switch {
			case c.Positive && !linear:
				t.Errorf("%s: %s: positive control classified %v", c.Name, s.name, got.Class)
			case !c.Positive && linear && s.h.Memo:
				t.Logf("%s: %s: classified %v, measured %v without the memo", c.Name, s.name, got.Class, c.Order)
			case !c.Positive && linear:
				t.Errorf("%s: %s: negative control classified %v, measured %v", c.Name, s.name, got.Class, c.Order)
			}
		}
	}
}
