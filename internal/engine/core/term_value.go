package core

import (
	"strings"

	"zeroparse.dev/zp/grammar"
	"zeroparse.dev/zp/internal/value"
)

// The value primitives of terms (ops.AddValue) of the engine. The compositions of the operators in
// term_exec.go decide which of them applies; each acts on one kind of value.

// binaryCode returns the code of the binary operator op, or false if op is not one of ops.BinaryOps.
// The codes are looked up for every term that is evaluated, so the lookup is a switch and not a search
// of the table; TestOperatorCodes holds it to the table.
func binaryCode(op string) (int, bool) {
	switch op {
	case "+":
		return OpAdd, true
	case "-":
		return OpSub, true
	case "*":
		return OpMul, true
	case "/":
		return OpQuo, true
	case "%":
		return OpRem, true
	case "==":
		return OpEq, true
	case "!=":
		return OpNe, true
	case "<":
		return OpLt, true
	case "<=":
		return OpLe, true
	case ">":
		return OpGt, true
	case ">=":
		return OpGe, true
	case "[]":
		return OpIndex, true
	}
	return 0, false
}

// unaryCode returns the code of the unary operator op, or false if op is not one of ops.UnaryOps.
func unaryCode(op string) (int, bool) {
	switch op {
	case "-":
		return OpNeg, true
	case "!":
		return OpNot, true
	}
	return 0, false
}

// logicalCode returns the code of the logical operator op, or false if op is not one of
// ops.LogicalOps.
func logicalCode(op string) (int, bool) {
	switch op {
	case "&&":
		return OpAnd, true
	case "||":
		return OpOr, true
	}
	return 0, false
}

// valKind is the operation val.kind: the kind of x. A node, a list, a tuple, a struct and a box are
// valKindOther; a box is opened by unboxed.
func valKind(x any) int {
	switch x.(type) {
	case nil:
		return valKindNil
	case int:
		return valKindInt
	case string:
		return valKindString
	case bool:
		return valKindBool
	}
	return valKindOther
}

// unboxed is the operation val.unbox: the value that a node pointer stands for: nil for a nil
// pointer, the value of a box, and otherwise v.
func unboxed(v any) any {
	if n, ok := v.(*value.Node); ok {
		if n == nil {
			return nil
		}
		if value.D(n).Kind == value.BoxKind {
			return unboxed(value.D(n).Box)
		}
	}
	return v
}

// intCmp is the operation int.cmp: -1, 0 or 1 as the int l is less than, equal to or greater than the int r.
func intCmp(l, r any) int {
	a, b := l.(int), r.(int)
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// intIndex is the operation int.index: the int x and true if it is an index of n elements.
func intIndex(x any, n int) (int, bool) {
	k := x.(int)
	if k < 0 || k >= n {
		return 0, false
	}
	return k, true
}

// valItems is the operation seq.items.
func valItems(x any) ([]*value.Node, bool) {
	switch x := x.(type) {
	case nil:
		return nil, true
	case *value.Node:
		if x == nil {
			return nil, true
		}
		return value.D(x).Children, true
	}
	return nil, false
}

// strAdd is the operation str.add. Under budget controls it copies the bytes in chunks, charging each.
func (p *Parser) strAdd(l, r any) any {
	ls, rs := l.(string), r.(string)
	if !BudgetControlsEnabled || p.Control == nil {
		return ls + rs
	}
	var out strings.Builder
	p.ChargeValueBytes(len(ls) + len(rs))
	out.Grow(len(ls) + len(rs))
	for _, s := range [...]string{ls, rs} {
		for off := 0; off < len(s); {
			n := min(spendValueChunk, len(s)-off)
			p.SpendValue(n)
			out.WriteString(s[off : off+n])
			off += n
		}
	}
	return out.String()
}

// strCmp is the operation str.cmp.
func (p *Parser) strCmp(l, r any) int {
	ls, rs := l.(string), r.(string)
	if !BudgetControlsEnabled || p.Control == nil {
		return strings.Compare(ls, rs)
	}
	return p.CompareText(ls, rs)
}

// strEq is the operation str.eq.
func (p *Parser) strEq(l, r any) bool {
	return p.equalText(l.(string), r.(string))
}

// nodeOf is the operation node.of: x as a node, list, tuple or struct.
func nodeOf(x any) (*value.Node, bool) {
	n, ok := x.(*value.Node)
	return n, ok && n != nil
}

// isStruct is the operation node.isStruct.
func (p *Parser) isStruct(n *value.Node) bool {
	_, ok := p.Prog.Types[n.Type()].(*grammar.TypeDef)
	return ok
}


