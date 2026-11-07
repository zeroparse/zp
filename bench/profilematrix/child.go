package main

// benchMain is the program that measures one generated parser in a process of its own. It is built in
// the module sizeprobe, which holds the parser as sizeprobe/gen, and run as
//
//	probe BENCHTIME FILE...
//
// It parses each FILE, keeps those that parse without an error, and prints one JSON object with
//
//	inputs     the number of files that were kept
//	ns         time of one pass over all inputs, in nanoseconds
//	bytes      bytes allocated by one pass
//	allocs     allocations of one pass
//	retained   live heap that the results of one pass hold, after three collections
//	gc_share   the share of the CPU time of the benchmark that went to garbage collection
//
// The parent measures the peak resident set size of the process.
const benchMain = `package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"runtime"
	"runtime/metrics"
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

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: probe BENCHTIME FILE...")
		os.Exit(2)
	}
	var inputs []string
	for _, f := range os.Args[2:] {
		data, err := os.ReadFile(f)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
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
	testing.Init()
	if err := flag.Set("test.benchtime", os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	gc0, total0 := cpu()
	r := testing.Benchmark(func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			for _, in := range inputs {
				p.ParseAST(in)
			}
		}
	})
	gc1, total1 := cpu()
	res.NS = float64(r.T.Nanoseconds()) / float64(r.N)
	res.Bytes = int64(r.MemBytes) / int64(r.N)
	res.Allocs = int64(r.MemAllocs) / int64(r.N)
	if total1 > total0 {
		res.GCShare = (gc1 - gc0) / (total1 - total0)
	}
	if err := json.NewEncoder(os.Stdout).Encode(res); err != nil {
		os.Exit(2)
	}
}
`
