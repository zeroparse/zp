package gentest

// UnsetZeroRules are the types of the tests of the fields that a constructor leaves unset, which
// read as the zero value of their declared type (spec/language/types.md#zero-values).
const UnsetZeroRules = `type In = { N int; S string }
type Z = { I int; S string; B bool; L []int; O *int; T (int, string); In In; A _ }
type Set = { I int; L []int }
rule z Z = "z" => Z{}
rule part Z = "p" => Z{I: 7, S: "s"}
`

// UnsetZeroCases returns the grammars and inputs of the tests of unset fields, which every backend
// runs: each field type that has a zero value is read from a struct that left it unset, and the value
// read is returned as it is and as the field of a struct, which the typed runtime reads back.
func UnsetZeroCases() []Case {
	var cases []Case
	for _, r := range []struct{ name, typ, action string }{
		{"int", "int", "$x.I"},
		{"string", "string", `$x.S`},
		{"bool", "bool", "$x.B"},
		{"list", "[]int", "$x.L"},
		{"list length", "int", "len($x.L)"},
		{"list in a concat", "[]int", "concat($x.L, []int{1})"},
		{"optional", "*int", "$x.O"},
		{"tuple", "(int, string)", "$x.T"},
		{"struct field", "int", "$x.In.N"},
		{"struct string field", "string", "$x.In.S"},
		{"struct", "In", "$x.In"},
		{"top", "_", "$x.A"},
		{"arithmetic", "int", "$x.I + $x.In.N + len($x.L)"},
		{"comparison", "bool", `$x.S == "" && $x.B == false`},
	} {
		cases = append(cases,
			Case{Name: r.name, Src: UnsetZeroRules + "rule main = x:z $$ => " + r.action + "\n", Inputs: []string{"z"}},
			Case{Name: r.name + " in a struct", Src: UnsetZeroRules + "type Out = { V " + r.typ + " }\nrule main Out = x:z $$ => Out{V: " + r.action + "}\n", Inputs: []string{"z"}})
	}
	return append(cases,
		Case{Name: "set and unset", Src: UnsetZeroRules + "rule main = x:part $$ => $x.I + len($x.S) + len($x.L)\n", Inputs: []string{"p"}},
		Case{Name: "whole struct", Src: UnsetZeroRules + "rule main = x:z $$ => $x\n", Inputs: []string{"z"}})
}
