package engine

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"unicode/utf8"

	"zeroparse.dev/zp/core"
	"zeroparse.dev/zp/internal/compile"
	"zeroparse.dev/zp/internal/engine/state"
	"zeroparse.dev/zp/internal/testing/codegentest"
)

// blockProbe wraps the reader of a stream parse and reports when the parser asks it for input after
// arm. A parser that holds everything it needs to emit an item does not read, so a read after the
// last byte that the test wrote means that the parser waits for future input or the end. The report
// takes the place of a timer, whose verdict depends on the load of the machine: it comes when the
// parser blocks, however long that takes, and never when the parser emits.
type blockProbe struct {
	r       io.Reader
	armed   atomic.Bool
	blocked chan struct{}
}

func newBlockProbe(r io.Reader) *blockProbe {
	return &blockProbe{r: r, blocked: make(chan struct{}, 1)}
}

func (p *blockProbe) Read(b []byte) (int, error) {
	if p.armed.Load() {
		select {
		case p.blocked <- struct{}{}:
		default:
		}
	}
	return p.r.Read(b)
}

// awaitEmitted returns the next value of emitted, which the parser sends from its callback, after
// the probe was armed. It fails with what if the parser reads again, or stops (its result is on
// done, and is put back for the caller), before it emitted: the callback runs before the parser
// can read again, so a value that is not there when the probe reports was never going to come.
func awaitEmitted[T any](t *testing.T, emitted <-chan T, probe *blockProbe, done chan error, what string) T {
	t.Helper()
	select {
	case v := <-emitted:
		return v
	case err := <-done:
		done <- err
		select {
		case v := <-emitted:
			return v
		default:
			t.Fatalf("the stream ended before the item was emitted: %v", err)
		}
	case <-probe.blocked:
		select {
		case v := <-emitted:
			return v
		default:
			t.Fatal(what)
		}
	}
	panic("unreachable")
}

// failOnRead lets decode read buffered bytes but reports every attempt to
// obtain unrelated data. The parser must not use read failures as EOF.
type failOnRead struct {
	calls int
	err   error
}

func (r *failOnRead) Read([]byte) (int, error) { r.calls++; return 0, r.err }

func TestByteDecodeDoesNotReadPastCompleteRune(t *testing.T) {
	t.Parallel()
	for _, s := range []string{"a", "é", "漢", "😀", string([]byte{0xff}), string([]byte{0xc3, '('})} {
		t.Run(fmt.Sprintf("%x", s), func(t *testing.T) {
			stop := errors.New("unrelated read")
			r := &failOnRead{err: stop}
			reader := bufio.NewReader(io.MultiReader(strings.NewReader(s), r))
			// Preload only this rune (or invalid prefix) without synthesizing EOF.
			if _, err := reader.Peek(len(s)); err != nil {
				t.Fatal(err)
			}
			in := state.Input{Unit: core.Bytes, Reader: reader}
			defer func() {
				if x := recover(); x != nil {
					t.Errorf("decode attempted unrelated read: %v", x)
				}
			}()
			got, size, examined, ok := in.Decode(0)
			want, wantSize := utf8.DecodeRuneInString(s)
			wantExamined := wantSize
			if want == utf8.RuneError && wantSize == 1 {
				wantExamined = len(s)
			}
			if !ok || size != wantSize || got != want || examined != wantExamined || r.calls != 0 || in.Eof {
				t.Fatalf("rune=%U size=%d ok=%v reads=%d eof=%v", got, size, ok, r.calls, in.Eof)
			}
		})
	}
}

func TestOpenPipeStreamEmitsCompleteItem(t *testing.T) {
	t.Parallel()
	for _, b := range []Backend{Closure, Bytecode, BytecodeIterative} {
		for _, unit := range []core.PositionUnit{core.CodePoints, core.Bytes} {
			for _, s := range []string{"a", "é", "漢", "😀", string([]byte{0xff})} {
				for split := 0; split < len(s); split++ {
					t.Run(fmt.Sprintf("%s/%s/%x/split%d", b, unit, s, split), func(t *testing.T) {
						p := mustCompile(t, "rule main = item* #stream $$\nrule item = . \"\\n\"")
						pr, pw := io.Pipe()
						probe := newBlockProbe(pr)
						emitted := make(chan string, 2)
						done := make(chan error, 1)
						go func() {
							defer pr.Close() // Unblock a writer if parsing fails before all chunks arrive.
							done <- p.ParseStreamWith("main", probe, func(n *core.Node) error { emitted <- codegentest.ResultJSON(n, nil); return nil }, ParseOptions{Backend: b, Unit: unit})
						}()
						t.Cleanup(func() {
							pw.Close()
							if t.Failed() {
								pr.Close()
							}
							// The end of the input or the closed reader ends the parse; a parse that does not
							// end is stopped by the test timeout, which names the goroutine.
							if err := <-done; !t.Failed() && err != nil {
								t.Errorf("stream completion: %v", err)
							}
							pr.Close()
						})
						// Each partial UTF-8 prefix arrives separately. Comparing the
						// final tree and completion detects premature replacement decoding.
						if split > 0 {
							if _, err := io.WriteString(pw, s[:split]); err != nil {
								t.Fatal(err)
							}
						}
						if _, err := io.WriteString(pw, s[split:]+"\n"); err != nil {
							t.Fatal(err)
						}
						probe.armed.Store(true)
						want := ""
						refErr := p.ParseStreamWith("main", strings.NewReader(s+"\n"), func(n *core.Node) error { want = codegentest.ResultJSON(n, nil); return nil }, ParseOptions{Unit: unit})
						if refErr != nil {
							t.Fatal(refErr)
						}
						if got := awaitEmitted(t, emitted, probe, done, "complete item waited for future bytes or EOF"); got != want {
							t.Fatalf("got %s want %s", got, want)
						}
						if err := pw.Close(); err != nil {
							t.Fatal(err)
						}
						// Cleanup owns the done receive, including failure paths.
					})
				}
			}
		}
	}
}

func TestByteDecodeIncompletePrefixAtEOF(t *testing.T) {
	t.Parallel()
	for _, s := range []string{"", "\xc3", "\xe6\xbc", "\xf0\x9f\x98"} {
		t.Run(fmt.Sprintf("%x", s), func(t *testing.T) {
			in := state.Input{Unit: core.Bytes, Reader: bufio.NewReader(strings.NewReader(s))}
			got, size, examined, ok := in.Decode(0)
			if s == "" {
				if ok || size != 0 || examined != 1 || !in.Eof {
					t.Fatalf("empty EOF: %U %d %d %v", got, size, examined, ok)
				}
			} else if !ok || got != utf8.RuneError || size != 1 || examined != len(s)+1 || !in.Eof {
				t.Fatalf("incomplete EOF: %U %d %d %v eof=%v", got, size, examined, ok, in.Eof)
			}
		})
	}
}

func TestStreamIncompletePrefixPropagatesReaderError(t *testing.T) {
	t.Parallel()
	p := mustCompile(t, "rule main = item* #stream $$\nrule item = . \"\\n\"")
	for _, backend := range []Backend{Closure, Bytecode, BytecodeIterative} {
		for _, unit := range []core.PositionUnit{core.CodePoints, core.Bytes} {
			t.Run(backend.String()+"/"+unit.String(), func(t *testing.T) {
				stop := errors.New("reader stopped")
				r := io.MultiReader(strings.NewReader("\xc3"), &failOnRead{err: stop})
				count := 0
				err := p.ParseStreamWith("main", r, func(*core.Node) error { count++; return nil }, ParseOptions{Backend: backend, Unit: unit})
				if !errors.Is(err, stop) || count != 0 {
					t.Fatalf("error=%v emitted=%d", err, count)
				}
			})
		}
	}
}

// BenchmarkRuneInputDelivery exercises the affected stream decoder and compares
// whole-input parsing, whose buffer already has a known end, in both units.
func BenchmarkRuneInputDelivery(b *testing.B) {
	g, err := parseTest("rule main = item* #stream $$\nrule item = . \"\\n\"")
	if err != nil {
		b.Fatal(err)
	}
	prog, err := Compile(g, compile.Options{})
	if err != nil {
		b.Fatal(err)
	}
	for _, tc := range []struct{ name, item string }{{"ASCII", "a\n"}, {"Unicode", "😀\n"}} {
		text := strings.Repeat(tc.item, 1000)
		for _, backend := range []Backend{Closure, Bytecode, BytecodeIterative} {
			if _, err := prog.rule(backend, "main", state.Inlined, nil); err != nil {
				b.Fatal(err)
			}
			for _, unit := range []core.PositionUnit{core.CodePoints, core.Bytes} {
				for _, stream := range []bool{true, false} {
					mode := "Batch"
					if stream {
						mode = "Stream"
					}
					b.Run(tc.name+"/"+backend.String()+"/"+unit.String()+"/"+mode, func(b *testing.B) {
						b.SetBytes(int64(len(text)))
						b.ReportAllocs()
						opts := ParseOptions{Backend: backend, Unit: unit}
						for b.Loop() {
							if stream {
								count := 0
								err := prog.ParseStreamWith("main", strings.NewReader(text), func(*core.Node) error { count++; return nil }, opts)
								if err != nil || count != 1000 {
									b.Fatalf("emitted %d: %v", count, err)
								}
							} else {
								if _, err := prog.ParseWith("main", text, opts); err != nil {
									b.Fatal(err)
								}
							}
						}
					})
				}
			}
		}
	}
}
