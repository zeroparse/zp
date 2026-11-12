package core

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// termOpNames names each instruction of term code as the specification's table does.
var termOpNames = map[string]termOp{
	"NOP": tNop, "CONST": tConst, "INT": tIntK, "MOVEA": tMoveA, "MOVEN": tMoveN, "MOVES": tMoveS,
	"MOVET": tMoveT, "CAP": tCap, "LOCAL": tLocal, "VAR": tVar, "UNBOX": tUnbox, "FROMB": tFromB,
	"FROMT": tFromT, "NOT": tNot, "LT": tLt, "ADD": tAdd, "EQN": tEqN, "ISNIL": tIsNil,
	"MEMBER": tMember, "UNARY": tUnary, "BINARY": tBinary, "LOGIC": tLogic, "TEXT": tText,
	"TEXTEQ": tTextEq, "LENGTH": tLength, "CONVERT": tConvert, "CONFORMS": tConforms,
	"CASTERR": tCastErr, "KIDS": tKids, "PUSH": tPush, "PUSHALL": tPushAll, "ABOVE": tAbove,
	"HULL": tHull, "ITEMS": tItems, "ITEMAT": tItemAt, "SEQLEN": tSeqLen, "STEP": tStep,
	"KEEP": tKeep, "FILTER": tFilter, "COPY": tCopy, "STRUCT": tStruct, "SETFIELD": tSetField,
	"COVER": tCover, "MADE": tMade, "JUMP": tJump, "JUMPF": tJumpF, "RET": tRet, "RETERR": tRetErr,
}

// TestSpecTermOpcodes checks that the opcodes of term code are those of the table in
// spec/bytecode/README.md: a module stores the opcodes as they are, so renumbering an instruction
// changes the file format.
func TestSpecTermOpcodes(t *testing.T) {
	data, err := os.ReadFile("../../../spec/bytecode/README.md")
	if err != nil {
		t.Fatal(err)
	}
	_, table, ok := strings.Cut(string(data), "\n## Term Code\n")
	if !ok {
		t.Fatal("spec/bytecode/README.md has no Term Code section")
	}
	table, _, _ = strings.Cut(table, "\n## ")
	row := regexp.MustCompile("(?m)^\\| (\\d+)(?:–(\\d+))? \\| ((?:`[A-Z]+`(?:, )?)+) \\|")
	name := regexp.MustCompile("`([A-Z]+)`")
	seen := map[string]bool{}
	for _, m := range row.FindAllStringSubmatch(table, -1) {
		first, _ := strconv.Atoi(m[1])
		last := first
		if m[2] != "" {
			last, _ = strconv.Atoi(m[2])
		}
		names := name.FindAllStringSubmatch(m[3], -1)
		if len(names) != last-first+1 {
			t.Errorf("the row of codes %s names %d instructions", m[0], len(names))
			continue
		}
		for i, n := range names {
			op, ok := termOpNames[n[1]]
			switch {
			case !ok:
				t.Errorf("the specification names the instruction %s, which the runtime does not have", n[1])
			case int(op) != first+i:
				t.Errorf("%s is code %d in the specification and %d in the runtime", n[1], first+i, op)
			}
			seen[n[1]] = true
		}
	}
	for n, op := range termOpNames {
		if !seen[n] {
			t.Errorf("the specification's table has no row for %s", n)
		}
		if _, ok := termOperands[op]; !ok {
			t.Errorf("%s has no operand kinds", n)
		}
	}
	if len(termOpNames) != len(termOperands) {
		t.Errorf("the runtime has %d instructions, the table names %d", len(termOperands), len(termOpNames))
	}
}
