package flowlower

import (
	"slices"
	"testing"
)

// TestTermLeavesWriteOwnComponent pins that the leaves of terms (addTermOps) read and write only their own
// component: what terms gather and make is the state of the components gathered and made, whose operations
// have reference models, and a leaf that wrote it would leave that state outside the models. Another
// writer needs a reason recorded here, as TestCrossComponentWriters of package ops records those of the
// parser VM.
func TestTermLeavesWriteOwnComponent(t *testing.T) {
	m, err := termModule()
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, o := range m.Ops {
		if o.Component != termComponent {
			continue
		}
		for _, s := range slices.Concat(o.Reads, o.Writes) {
			if s != termComponent {
				got = append(got, o.Name+" uses "+s)
			}
		}
	}
	if len(got) != 0 {
		t.Errorf("leaves of terms that use another component: %q", got)
	}
}
