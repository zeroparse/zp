package core

import (
	"errors"
	"iter"
)

// ErrStreamClosed is returned after an iterator is abandoned before termination.
var ErrStreamClosed = errors.New("stream iterator is closed")

// StreamIterator advances synchronously to one committed element at a time.
// It is single-owner: Next and Close must not overlap, including callbacks.
// Close releases parser state but never closes or interrupts its reader.
// The standard library pull coroutine has no independently scheduled producer.
type StreamIterator struct {
	next     func() (*Node, bool)
	stop     func()
	terminal bool
	err      error
}

// NewStreamIterator returns an iterator over the elements that run yields. run parses a stream,
// passes each committed element to yield, and returns the error that ends the parse, or nil; once
// yield returns false it must stop and return. run starts with the first call of Next. Parsers
// use it to offer an iterator; programs get one from a parser.
func NewStreamIterator(run func(yield func(*Node) bool) error) *StreamIterator {
	it := &StreamIterator{}
	it.next, it.stop = iter.Pull(func(yield func(*Node) bool) {
		it.err = run(yield)
	})
	return it
}

// Next returns (node,true,nil), even for a successful discarded nil value.
// Clean EOF is (nil,false,nil); terminal errors remain stable on later calls.
func (it *StreamIterator) Next() (n *Node, ok bool, err error) {
	if it.terminal {
		return nil, false, it.err
	}
	defer func() {
		if x := recover(); x != nil {
			// Pull propagates the original caller panic after the parser unwinds.
			it.terminal = true
			it.next, it.stop = nil, nil
			panic(x)
		}
	}()
	n, ok = it.next()
	if ok {
		return n, true, nil
	}
	it.terminal = true
	it.next, it.stop = nil, nil
	return nil, false, it.err
}

// Close abandons unfinished parsing and is idempotent. It cannot interrupt a
// Read already blocked in caller code, and must not overlap a call to Next.
func (it *StreamIterator) Close() {
	if it.terminal {
		return
	}
	it.terminal = true
	stop := it.stop
	it.next, it.stop = nil, nil
	defer func() { it.err = ErrStreamClosed }()
	stop()
}
