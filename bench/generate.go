// Package bench benchmarks the ZeroParse backends against each other and against Go standard library parsers.
//
//	go test ./bench -bench . -benchmem
//
// docs/benchmarks.md is generated from a run of these benchmarks: go run ./bench/report measures them
// and rewrites it (see package report).
//
// The generated parsers that the benchmarks run are in internal/bench/gen, written by zp gen;
// regenerate them with go generate ./bench. Document (minilangdoc, minilangtdoc, csvdoc) and stream (csvstream)
// parsers are separate packages, so the batch parsers keep their generated form. The inputs come from internal/bench/inputs.
package bench

//go:generate go run ../cmd/zp gen -g ../testdata/grammars/json/json.zp -module zeroparse.dev/zp/internal/bench/gen/json -lib import -recognize -types -limits -o ../internal/bench/gen/json
//go:generate go run ../cmd/zp gen -g ../testdata/grammars/csv/csv.zp -module zeroparse.dev/zp/internal/bench/gen/csv -lib import -recognize -types -limits -o ../internal/bench/gen/csv
//go:generate go run ../cmd/zp gen -g ../testdata/grammars/xml/xml.zp -module zeroparse.dev/zp/internal/bench/gen/xml -lib import -recognize -types -limits -o ../internal/bench/gen/xml
//go:generate go run ../cmd/zp gen -g ../examples/calculator/calc.zp -module zeroparse.dev/zp/internal/bench/gen/calc -lib import -recognize -types -limits -o ../internal/bench/gen/calc
//go:generate go run ../cmd/zp gen -g ../examples/calculator/calc_lr.zp -module zeroparse.dev/zp/internal/bench/gen/calclr -lib import -recognize -types -limits -o ../internal/bench/gen/calclr
//go:generate go run ../cmd/zp gen -g ../examples/minilang/minilang.zp -module zeroparse.dev/zp/internal/bench/gen/minilang -lib import -recognize -types -limits -o ../internal/bench/gen/minilang
//go:generate go run ../cmd/zp gen -g ../examples/outline/outline.zp -module zeroparse.dev/zp/internal/bench/gen/outline -lib import -recognize -types -limits -o ../internal/bench/gen/outline
//go:generate go run ../cmd/zp gen -g grammars/terms/pred.zp -module zeroparse.dev/zp/internal/bench/gen/termspred -lib import -recognize -types -limits -o ../internal/bench/gen/termspred
//go:generate go run ../cmd/zp gen -g grammars/terms/heavy.zp -module zeroparse.dev/zp/internal/bench/gen/termsheavy -lib import -recognize -types -limits -o ../internal/bench/gen/termsheavy
//go:generate go run ../cmd/zp gen -g ../examples/minilang/minilang.zp -module zeroparse.dev/zp/internal/bench/gen/minilangdoc -lib import -limits -document -o ../internal/bench/gen/minilangdoc
//go:generate go run ../cmd/zp gen -g ../examples/minilang/minilang.zp -module zeroparse.dev/zp/internal/bench/gen/minilangtdoc -lib import -types -limits -document -o ../internal/bench/gen/minilangtdoc
//go:generate go run ../cmd/zp gen -g ../testdata/grammars/csv/csv.zp -module zeroparse.dev/zp/internal/bench/gen/csvdoc -lib import -limits -document -o ../internal/bench/gen/csvdoc
//go:generate go run ../cmd/zp gen -g ../testdata/grammars/csv/csv.zp -module zeroparse.dev/zp/internal/bench/gen/csvstream -lib import -limits -stream -o ../internal/bench/gen/csvstream
