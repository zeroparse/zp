package compile

import (
	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/check"
)

// LookKeepsState is check.LookKeepsState.
func LookKeepsState(e grammar.Expr) bool { return check.LookKeepsState(e) }
