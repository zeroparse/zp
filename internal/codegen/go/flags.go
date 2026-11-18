package gogen

import "zeroparse.dev/zp/internal/pass"

// The flags that only the Go printer reads.
func init() {
	pass.Register(
		pass.Flag{Name: "field-compare", Kind: pass.KindLowering, Default: 1, Doc: "typed Go compares fields in the actions and predicates of typed rules without boxing string fields"},
		pass.Flag{Name: "class-tables", Kind: pass.KindLowering, Int: true, Default: 0, Doc: "match a class of at least this many ranges through a membership table (lean typed Go); smaller source against a slower parse, so an opt-in: 0 in speed, 5 is the measured choice"},
		pass.Flag{Name: "typed-object-slabs", Kind: pass.KindRuntime, Default: 1, Doc: "allocate typed objects in slabs (typed Go)"},
		pass.Flag{Name: "typed-list-slabs", Kind: pass.KindRuntime, Default: 1, Doc: "allocate typed lists in slabs (typed Go)"},
		pass.Flag{Name: "typed-output-bootstrap", Kind: pass.KindRuntime, Default: 1, Doc: "size the first slab of the typed output from the input (typed Go)"},
	)
}
