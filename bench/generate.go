// Package bench benchmarks the ZeroParse backends against each other and against Go standard library parsers.
//
//	go test ./bench -bench . -benchmem
//
// docs/benchmarks.md is generated from a run of these benchmarks: go run ./bench/report measures them
// and rewrites it (see package report).
//
// The generated parsers that the benchmarks run are in internal/bench/parsers, written by zp gen;
// regenerate them with go generate ./bench. Document (minilangdoc, minilangtdoc, csvdoc) and stream (csvstream)
// parsers are separate packages, so the batch parsers keep their generated form. The inputs come from internal/bench/inputs.
package bench

//go:generate go run ../cmd/zp gen -g ../testdata/grammars/json/json.zp -module zeroparse.dev/zp/internal/bench/parsers/json -lib import -recognize -types -limits -o ../internal/bench/parsers/json
//go:generate go run ../cmd/zp gen -g ../testdata/grammars/csv/csv.zp -module zeroparse.dev/zp/internal/bench/parsers/csv -lib import -recognize -types -limits -o ../internal/bench/parsers/csv
//go:generate go run ../cmd/zp gen -g ../testdata/grammars/xml/xml.zp -module zeroparse.dev/zp/internal/bench/parsers/xml -lib import -recognize -types -limits -o ../internal/bench/parsers/xml
//go:generate go run ../cmd/zp gen -g ../examples/calculator/calc.zp -module zeroparse.dev/zp/internal/bench/parsers/calc -lib import -recognize -types -limits -o ../internal/bench/parsers/calc
//go:generate go run ../cmd/zp gen -g ../examples/calculator/calc_lr.zp -module zeroparse.dev/zp/internal/bench/parsers/calclr -lib import -recognize -types -limits -o ../internal/bench/parsers/calclr
//go:generate go run ../cmd/zp gen -g ../examples/minilang/minilang.zp -module zeroparse.dev/zp/internal/bench/parsers/minilang -lib import -recognize -types -limits -o ../internal/bench/parsers/minilang
//go:generate go run ../cmd/zp gen -g ../examples/outline/outline.zp -module zeroparse.dev/zp/internal/bench/parsers/outline -lib import -recognize -types -limits -o ../internal/bench/parsers/outline
//go:generate go run ../cmd/zp gen -g grammars/terms/pred.zp -module zeroparse.dev/zp/internal/bench/parsers/termspred -lib import -recognize -types -limits -o ../internal/bench/parsers/termspred
//go:generate go run ../cmd/zp gen -g grammars/terms/heavy.zp -module zeroparse.dev/zp/internal/bench/parsers/termsheavy -lib import -recognize -types -limits -o ../internal/bench/parsers/termsheavy
//go:generate go run ../cmd/zp gen -g ../examples/minilang/minilang.zp -module zeroparse.dev/zp/internal/bench/parsers/minilangdoc -lib import -limits -document -o ../internal/bench/parsers/minilangdoc
//go:generate go run ../cmd/zp gen -g ../examples/minilang/minilang.zp -module zeroparse.dev/zp/internal/bench/parsers/minilangtdoc -lib import -types -limits -document -o ../internal/bench/parsers/minilangtdoc
//go:generate go run ../cmd/zp gen -g ../testdata/grammars/csv/csv.zp -module zeroparse.dev/zp/internal/bench/parsers/csvdoc -lib import -limits -document -o ../internal/bench/parsers/csvdoc
//go:generate go run ../cmd/zp gen -g ../testdata/grammars/csv/csv.zp -module zeroparse.dev/zp/internal/bench/parsers/csvstream -lib import -limits -stream -o ../internal/bench/parsers/csvstream
