package engine

import (
	"errors"
	"fmt"
	"io"

	"zeroparse.dev/zp/internal/engine/core"
	"zeroparse.dev/zp/runtime/parse"
)

var errIteratorStop = errors.New("stream iterator stopped")

// Iterate validates the entry point before returning a lazy iterator. Reader
// access starts with Next. Parser lookahead and buffering may read ahead.
func (prog *Program) Iterate(start string, r io.Reader, o ParseOptions) (it *parse.StreamIterator, err error) {
	defer func() {
		if x := recover(); x != nil {
			if f, ok := x.(core.Fatal); ok {
				it, err = nil, f.Err
			} else if cp, ok := x.(core.ContextPanic); ok {
				panic(cp.Value)
			} else {
				panic(x)
			}
		}
	}()
	if err := core.ValidateStreamDiagnostics(o, true); err != nil {
		return nil, err
	}
	if o.Recognize {
		return nil, fmt.Errorf("recognition is not supported for stream parsing")
	}
	start = prog.ResolveStart(start) // an empty name selects the default start rule
	if err := prog.checkMode(o, false); err != nil {
		return nil, err
	}
	if _, _, _, err = prog.admitParse(start, o, optionsFamily(o), true, ""); err != nil {
		return nil, err
	}
	return parse.NewStreamIterator(func(yield func(*parse.Node) bool) error {
		return prog.ParseStreamWith(start, r, func(n *parse.Node) error {
			if !yield(n) {
				return errIteratorStop
			}
			return nil
		}, o)
	}), nil
}
