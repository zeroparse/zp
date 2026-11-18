package corepass

import "zeroparse.dev/zp/internal/pass"

// The Core passes register their flags under the names of the passes.
func init() {
	pass.Register(
		pass.Flag{Name: PassGuards, Kind: pass.KindCorePass, Default: 1, Doc: "skip an alternative when the next character cannot start it"},
		pass.Flag{Name: PassScans, Kind: pass.KindCorePass, Default: 1, Doc: "a value-free repetition of a class or of any character runs as a scan"},
	)
}
