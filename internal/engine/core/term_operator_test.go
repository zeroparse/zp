package core

import (
	"strings"
	"testing"

	"zeroparse.dev/zp/internal/ir/exec/ops"
)

// TestOperatorCodes holds the lookups of the codes of operators to the tables of operators, whose
// index is the code.
func TestOperatorCodes(t *testing.T) {
	for _, c := range []struct {
		name string
		list []string
		code func(string) (int, bool)
	}{{"binary", ops.BinaryOps, binaryCode}, {"unary", ops.UnaryOps, unaryCode}, {"logical", ops.LogicalOps, logicalCode}} {
		for i, op := range c.list {
			if got, ok := c.code(op); !ok || got != i {
				t.Errorf("%s operator %q: code %d, %v; want %d", c.name, op, got, ok, i)
			}
		}
	}
}

// TestUnknownOperatorIsAnError checks that an operator that no table of operators has, as only a
// corrupted program carries, fails the term and is not taken for another operator.
func TestUnknownOperatorIsAnError(t *testing.T) {
	p := &Parser{}
	check := func(what string, err error) {
		t.Helper()
		if err == nil || !strings.Contains(err.Error(), "unknown operator") {
			t.Errorf("%s: err = %v, want an unknown operator error", what, err)
		}
	}
	_, err := p.ValueBinary("**", 1, 2)
	check("binary", err)
	_, err = p.ValueBinary("&&", true, true)
	check("binary with a logical operator", err)
	_, err = p.ValueUnary("~", 1)
	check("unary", err)
	_, err = p.ValueUnary("&&", true)
	check("unary with a logical operator", err)
	_, err = p.ValueLogic("^", true)
	check("logic", err)
	_, err = p.ValueLogic("+", true)
	check("logic with a binary operator", err)
	if v, err := p.ValueBinary("+", 1, 2); err != nil || v != 3 {
		t.Errorf("1 + 2 = %v, %v", v, err)
	}
}
