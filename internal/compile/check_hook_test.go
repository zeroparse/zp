package compile_test

import (
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/frontend"
)

func init() { compile.CheckGrammar = frontend.Check }
