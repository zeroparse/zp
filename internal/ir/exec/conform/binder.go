package conform

import (
	"fmt"
	"reflect"
	"strings"

	"zeroparse.dev/zp/internal/ir/exec"
	"zeroparse.dev/zp/internal/ir/exec/ref"
)

// A Converter converts between the handles of the conformance harness and the Go values that the
// bindings of a target take and return.
type Converter interface {
	// ToImpl returns the Go value of type t for the argument arg, and false if the converter does not
	// convert it: integers, booleans, the null handle (the zero value of t) and handles that are
	// themselves the target's values need no conversion.
	ToImpl(arg any, t reflect.Type) (reflect.Value, bool)
	// FromImpl returns the handle of the harness for the Go value v of a result, and false if the
	// value is its own handle.
	FromImpl(v reflect.Value) (any, bool)
}

// Binder calls the printed bindings of a target (named as internal/codegen/go/lib names them) by
// reflection, so that one harness serves the engine and the runtime. For the records of the split
// forms of the Operators Rule loop it keeps the handle that each field was last set to, and returns
// it from the getter when the binding returns the value that was set; any other value is a new
// handle.
type Binder struct {
	// Methods are the bindings of the target by operation name, as the printed table returns them.
	Methods map[string]any
	Conv    Converter
	set     map[[2]any]any
}

// Call runs the binding of the operation o with the arguments args.
func (b *Binder) Call(o *exec.Op, args []any) []any {
	fn, ok := b.Methods[o.Name]
	if !ok {
		panic("no binding for " + o.Name)
	}
	method := reflect.ValueOf(fn)
	in := make([]reflect.Value, len(args))
	for i, a := range args {
		in[i] = b.toImpl(a, method.Type().In(i))
	}
	out := method.Call(in)
	res := make([]any, len(out))
	for i, v := range out {
		switch v.Kind() {
		case reflect.Int:
			res[i] = v.Int()
		case reflect.Bool:
			res[i] = v.Bool()
		default:
			if h, ok := b.Conv.FromImpl(v); ok {
				res[i] = h
			} else if v.Kind() == reflect.Pointer && v.IsNil() {
				res[i] = nil
			} else {
				res[i] = v.Interface()
			}
		}
	}
	if rec, field, set := recordOp(o.Name); rec != "" {
		if b.set == nil {
			b.set = map[[2]any]any{}
		}
		key := [2]any{args[0], field}
		switch {
		case set:
			b.set[key] = args[1]
		case len(res) == 1:
			if h, ok := b.set[key]; ok && reflect.DeepEqual(b.toImpl(h, out[0].Type()).Interface(), out[0].Interface()) {
				res[0] = h
			}
		}
	}
	return res
}

func (b *Binder) toImpl(a any, t reflect.Type) reflect.Value {
	switch t.Kind() {
	case reflect.Int:
		return reflect.ValueOf(int(a.(int64)))
	case reflect.Bool:
		return reflect.ValueOf(a.(bool))
	}
	if v, ok := b.Conv.ToImpl(a, t); ok {
		return v
	}
	if a == nil {
		return reflect.Zero(t)
	}
	v := reflect.ValueOf(a)
	if !v.Type().AssignableTo(t) {
		panic(fmt.Sprintf("binding argument %T is not a %s", a, t))
	}
	return v
}

// recordOp returns the prefix of the record that the operation name belongs to, its field and whether
// the operation sets it; the prefix is empty for another operation.
func recordOp(name string) (rec, field string, set bool) {
	rec, f, _ := strings.Cut(name, ".")
	if _, ok := ref.OprRecords[rec]; !ok {
		return "", "", false
	}
	if x, ok := strings.CutPrefix(f, "set"); ok && x != "" && x[0] >= 'A' && x[0] <= 'Z' {
		return rec, strings.ToLower(x[:1]) + x[1:], true
	}
	return rec, f, false
}
