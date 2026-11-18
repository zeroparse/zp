package corepass

import "zeroparse.dev/zp/internal/pass"

// The Core passes register their flags: each pass under its name, and the variants of a pass under
// the names of their flags.
func init() {
	pass.Register(
		pass.Flag{Name: PassTerminals, Kind: pass.KindCorePass, Default: 1, Doc: "value-free choices of many literals and classes run as one program (lean typed Go)"},
		pass.Flag{Name: PassGuards, Kind: pass.KindCorePass, Default: 1, Doc: "skip an alternative when the next character cannot start it"},
		pass.Flag{Name: FlagGuardedClasses, Kind: pass.KindCorePass, Default: 1, Doc: "a leading class of a guarded alternative consumes the character its guard read (ordinary Go parsers)"},
		pass.Flag{Name: PassScans, Kind: pass.KindCorePass, Default: 1, Doc: "a value-free repetition of a class or of any character runs as a scan"},
		pass.Flag{Name: PassClassRuns, Kind: pass.KindCorePass, Default: 1, Doc: "consecutive classes of a value-free sequence run as one (lean typed Go)"},
	)
}
