// Command fiximports rewrites the import blocks of Go files to the repository rule:
// at most one blank line, between the standard library group and the rest, each
// group sorted by import path. Run it from the module root:
//
//	hack/fix-imports.sh [-n] [path ...]
//
// Without paths it rewrites every Go file that git tracks. With -n it only lists the
// files it would change. Generated files are listed with a "generated" mark: fix the
// generator and regenerate instead of keeping the rewrite.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"zeroparse.dev/zp/internal/testing/importgroups"
)

func main() {
	dry := flag.Bool("n", false, "list files that would change; write nothing")
	flag.Parse()
	paths := flag.Args()
	if len(paths) == 0 {
		out, err := exec.Command("git", "ls-files", "-z", "*.go").Output()
		if err != nil {
			fmt.Fprintln(os.Stderr, "git ls-files:", err)
			os.Exit(2)
		}
		for p := range strings.SplitSeq(string(out), "\x00") {
			if p != "" {
				paths = append(paths, p)
			}
		}
	}
	status := 0
	for _, p := range paths {
		src, err := os.ReadFile(p)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			status = 2
			continue
		}
		fixed, skipped, err := importgroups.Fix(p, src)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", p, err)
			status = 2
			continue
		}
		for _, s := range skipped {
			fmt.Fprintf(os.Stderr, "%s: %s: not rewritten (detached comment between imports); fix by hand\n", p, s)
			status = 1
		}
		if bytes.Equal(fixed, src) {
			continue
		}
		mark := ""
		if bytes.Contains(src[:min(len(src), 2048)], []byte("Code generated")) {
			mark = " (generated)"
		}
		fmt.Println(p + mark)
		if !*dry {
			if err := os.WriteFile(p, fixed, 0o644); err != nil {
				fmt.Fprintln(os.Stderr, err)
				status = 2
			}
		}
	}
	os.Exit(status)
}
