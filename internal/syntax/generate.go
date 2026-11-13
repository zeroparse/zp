package syntax

// The generated parser of the grammar source, from the grammar of the language (zp.zp). It is
// checked in, so that building ZeroParse needs no generation step; a test of spec/language fails
// when it is stale.
//go:generate go run ../../cmd/zp gen -g ../../zp.zp -module zeroparse.dev/zp/internal/syntax/genparser -lib import -types -typed-only -limits -expectations -o genparser
