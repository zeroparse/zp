package gentest

// SeparatedCases returns the grammars and inputs of the separated list (#sep) tests, which every
// backend runs. The inputs include the lists that the policies accept and reject.
func SeparatedCases() []Case {
	const nums = "rule num = [0-9]+\n"
	inputs := []string{"", "1", "1,2,3", "1,", "1,2,", ",", "1,,2", "1 ,2", "1,2;", "1,2,3,4,5", "12,345"}
	return []Case{
		{Name: "forbidden", Src: "rule main = n:num* #sep(\",\") $$\n" + nums, Inputs: inputs},
		{Name: "forbidden one or more", Src: "rule main = num+ #sep(\",\") $$\n" + nums, Inputs: inputs},
		{Name: "optional trailing", Src: "rule main = num* #sep(\",\", trailing=\"optional\") $$\n" + nums, Inputs: inputs},
		{Name: "optional trailing one or more", Src: "rule main = num+ #sep(\",\", trailing=\"optional\") $$\n" + nums, Inputs: inputs},
		{Name: "required trailing", Src: "rule main = num* #sep(\",\", trailing=\"required\") $$\n" + nums, Inputs: inputs},
		{Name: "bounds", Src: "rule main = num #repeat(2, 3) #sep(\",\") $$\n" + nums, Inputs: inputs},
		{Name: "bounds optional trailing", Src: "rule main = num #repeat(2, 3) #sep(\",\", trailing=\"optional\") $$\n" + nums, Inputs: inputs},
		{Name: "element captures", Src: "rule main = (a:num \":\" b:num)* #sep(\",\") $$\n" + nums, Inputs: []string{"", "1:2", "1:2,3:4", "1:2,", "1:,3:4"}},
		{Name: "element value", Src: "type T = { A []node[string] }\nrule main = xs:([a-z]+ #atomic)* #sep(\",\") $$ => T{A: $xs.Value}\n", Inputs: []string{"", "a", "a,bc,d", "a,"}},
		{Name: "separator rule", Src: "rule main = \"[\" num* #sep(comma) \"]\" $$\nrule comma = ws \",\" ws\nrule ws = [ ]*\n" + nums, Inputs: []string{"[]", "[1]", "[1 , 2,3]", "[1,]", "[1 2]", "[ ]"}},
		{Name: "nested", Src: "rule main = list $$\nrule list = \"[\" item* #sep(\",\") \"]\"\nrule item = num / list\n" + nums, Inputs: []string{"[]", "[1,[2,3],[]]", "[1,[2,3]", "[1,,2]", "[[[]]]"}},
		{Name: "cut in the rule", Src: "rule main = a / b\nrule a = \"x\" #cut num* #sep(\",\") $$\nrule b = \"x\" .*\n" + nums, Inputs: []string{"x1,2", "x1,", "x", "xy"}},
		{Name: "failed list tries the next alternative", Src: "rule main = (num* #sep(\",\") \";\" / [0-9,;]+) $$\n" + nums, Inputs: []string{"1,2;", "1,;", "1,2", ",;"}},
		{Name: "choice element", Src: "rule main = (num / word)+ #sep(\",\") $$\nrule word = [a-z]+\n" + nums, Inputs: []string{"1,a,22,bc", "1,", "a1"}},
		{Name: "value-free list", Src: "rule main = &(num* #sep(\",\") \";\") [0-9,;]+ $$\n" + nums, Inputs: []string{"1,2;", "1,;", "1,2"}},
	}
}
