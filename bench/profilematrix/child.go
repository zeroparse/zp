package main

// The programs of this file measure one generated parser in a process of its own. They are built in
// the module sizeprobe, which holds the parser as sizeprobe/gen, and run as
//
//	probe BENCHTIME FILE...
//
// and print one JSON object with
//
//	inputs     the number of files that were kept
//	ns         time of one pass (or, for the incremental program, of one edit and reparse), in nanoseconds
//	bytes      bytes allocated by one pass
//	allocs     allocations of one pass
//	retained   live heap that the results of one pass hold, after three collections (0 where not measured)
//	gc_share   the share of the CPU time of the benchmark that went to garbage collection
//
// benchMain parses each FILE, keeps those that parse without an error, and times a pass over all of
// them. incrementalMain and streamMain take the one FILE of a CSV input and time, respectively, an
// edit in the middle followed by a reparse of a Document, and the parse of the whole input as a
// stream of records. The parent measures the peak resident set size of the process.
const benchHead = `package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"runtime"
	"runtime/metrics"
	"strings"
	"testing"

	p "sizeprobe/gen"
)

type result struct {
	Inputs   int     ` + "`json:\"inputs\"`" + `
	NS       float64 ` + "`json:\"ns\"`" + `
	Bytes    int64   ` + "`json:\"bytes\"`" + `
	Allocs   int64   ` + "`json:\"allocs\"`" + `
	Retained int64   ` + "`json:\"retained\"`" + `
	GCShare  float64 ` + "`json:\"gc_share\"`" + `
}

func heap() uint64 {
	for i := 0; i < 3; i++ {
		runtime.GC()
	}
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapAlloc
}

func cpu() (gc, total float64) {
	s := []metrics.Sample{{Name: "/cpu/classes/gc/total:cpu-seconds"}, {Name: "/cpu/classes/total:cpu-seconds"}}
	metrics.Read(s)
	return s[0].Value.Float64(), s[1].Value.Float64()
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(2)
}

// bench runs f under testing.Benchmark and fills the time, allocation and collection fields of res.
func bench(res *result, f func()) {
	testing.Init()
	if err := flag.Set("test.benchtime", os.Args[1]); err != nil {
		fail(err)
	}
	gc0, total0 := cpu()
	r := testing.Benchmark(func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			f()
		}
	})
	gc1, total1 := cpu()
	res.NS = float64(r.T.Nanoseconds()) / float64(r.N)
	res.Bytes = int64(r.MemBytes) / int64(r.N)
	res.Allocs = int64(r.MemAllocs) / int64(r.N)
	if total1 > total0 {
		res.GCShare = (gc1 - gc0) / (total1 - total0)
	}
}

func emit(res result) {
	if err := json.NewEncoder(os.Stdout).Encode(res); err != nil {
		os.Exit(2)
	}
}

func input() string {
	if len(os.Args) != 3 {
		fail(fmt.Errorf("usage: probe BENCHTIME FILE"))
	}
	data, err := os.ReadFile(os.Args[2])
	if err != nil {
		fail(err)
	}
	return string(data)
}

var _ = strings.NewReader
`

const benchMain = benchHead + `
func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: probe BENCHTIME FILE...")
		os.Exit(2)
	}
	var inputs []string
	for _, f := range os.Args[2:] {
		data, err := os.ReadFile(f)
		if err != nil {
			fail(err)
		}
		if _, err := p.ParseAST(string(data)); err == nil {
			inputs = append(inputs, string(data))
		}
	}
	if len(inputs) == 0 {
		fmt.Fprintln(os.Stderr, "no input parses without an error")
		os.Exit(2)
	}
	res := result{Inputs: len(inputs)}
	before := heap()
	held := make([]any, len(inputs))
	for i, in := range inputs {
		held[i], _ = p.ParseAST(in)
	}
	res.Retained = int64(heap()) - int64(before)
	runtime.KeepAlive(held)
	held = nil
	bench(&res, func() {
		for _, in := range inputs {
			p.ParseAST(in)
		}
	})
	emit(res)
}
`

const incrementalMain = benchHead + `
func main() {
	in := input()
	at := strings.Index(in, "\n25000,") + len("\n25000,")
	if at < len("\n25000,") {
		fail(fmt.Errorf("the input has no record 25000"))
	}
	doc, err := p.NewDocument(in)
	if err != nil {
		fail(err)
	}
	if _, err := doc.ParseAST(); err != nil {
		fail(err)
	}
	res := result{Inputs: 1}
	i := 0
	bench(&res, func() {
		var err error
		if i%2 == 0 {
			err = doc.Edit(at, at, "z")
		} else {
			err = doc.Edit(at, at+1, "")
		}
		i++
		if err != nil {
			fail(err)
		}
		if _, err := doc.ParseAST(); err != nil {
			fail(err)
		}
	})
	emit(res)
}
`

const streamMain = benchHead + `
func main() {
	in := input()
	res := result{Inputs: 1}
	bench(&res, func() {
		n := 0
		if err := p.ParseStreamAST(strings.NewReader(in), func(p.StreamValue) error { n++; return nil }); err != nil || n != 50001 {
			fail(fmt.Errorf("%d records: %v", n, err))
		}
	})
	emit(res)
}
`
