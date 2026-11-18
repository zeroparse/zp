package flowpass

import "zeroparse.dev/zp/internal/pass"

// The Flow passes register their flags under the names of the passes.
func init() {
	pass.Register(
		pass.Flag{Name: PassTerminals, Kind: pass.KindFlowPass, Default: 1, Doc: "value-free choices of many literals and classes run as one program (lean typed Go)"},
		pass.Flag{Name: PassGuardedClasses, Kind: pass.KindFlowPass, Default: 1, Doc: "a leading class of a guarded alternative consumes the character its guard read (ordinary Go parsers)"},
		pass.Flag{Name: PassClassRuns, Kind: pass.KindFlowPass, Default: 1, Doc: "consecutive classes of a value-free sequence run as one (lean typed Go)"},
		pass.Flag{Name: PassClassTables, Kind: pass.KindFlowPass, Int: true, Default: 0, Doc: "match a class of at least this many ranges through a membership table (lean typed Go); smaller source against a slower parse, so an opt-in: 0 in speed, 5 is the measured choice"},
		pass.Flag{Name: PassInlineLiterals, Kind: pass.KindFlowPass, Int: true, Default: 4, Doc: "compare literals of at most this many code points in place first; 0 turns it off"},
	)
}
