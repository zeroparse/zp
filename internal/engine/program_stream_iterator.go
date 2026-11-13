package engine

import (
	"errors"
	"fmt"
	"io"

	"zeroparse.dev/zp/core"
	ecore "zeroparse.dev/zp/internal/engine/core"
)

var errIteratorStop = errors.New("stream iterator stopped")

// Iterate validates the entry point before returning a lazy iterator. Reader
// access starts with Next. Parser lookahead and buffering may read ahead.
func (prog *Program) Iterate(start string, r io.Reader, o ParseOptions) (it *core.StreamIterator, err error) {
	defer func() {
		if x := recover(); x != nil {
			if f, ok := x.(ecore.Fatal); ok {
				it, err = nil, f.Err
			} else if cp, ok := x.(ecore.ContextPanic); ok {
				panic(cp.Value)
			} else {
				panic(x)
			}
		}
	}()
	if err := ecore.ValidateStreamDiagnostics(o, true); err != nil {
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
	return core.NewStreamIterator(func(yield func(*core.Node) bool) error {
		return prog.ParseStreamWith(start, r, func(n *core.Node) error {
			if !yield(n) {
				return errIteratorStop
			}
			return nil
		}, o)
	}), nil
}
