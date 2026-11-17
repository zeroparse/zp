package fmt

import "context"

// formatControl polls ctx while one formatting call runs. One control is shared by the whole
// call, so many small pieces cannot hide an unbounded traversal between cancellation checks.
type formatControl struct {
	ctx   context.Context
	steps uint32
}

type formatCancellation struct{ err error }

func (c *formatControl) poll() {
	if c != nil {
		c.steps++
		if c.steps%256 == 0 {
			c.check()
		}
	}
}

func (c *formatControl) check() {
	if err := c.ctx.Err(); err != nil {
		panic(formatCancellation{err})
	}
}

// recoverFormat, deferred by an entry point, translates only this package's cancellation unwind
// and discards the partial result.
func recoverFormat[T any](result *T, err *error) {
	if failure := recover(); failure != nil {
		if stopped, ok := failure.(formatCancellation); ok {
			var zero T
			*result, *err = zero, stopped.err
		} else {
			panic(failure)
		}
	}
}

func appendFormat[T any](dst, src []T, control *formatControl) []T {
	if control == nil {
		return append(dst, src...)
	}
	for _, item := range src {
		control.poll()
		dst = append(dst, item)
	}
	return dst
}
