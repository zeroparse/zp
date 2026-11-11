package complexitytest

// nestedPattern matches the reasons for the class unknown whose units are nested without bound
// through a recursion of rule calls, other than at a #recover base.
const nestedPattern = `^(the alternative|the option|an iteration|the operator part|the operand|the lookahead|the expression)[^#]* is not committing against .*, and units that are not committing are nested in it without bound through a recursion of rule calls$`

// nestsLate matches the reason for a call of callee in rule caller, each a regular expression,
// whose evaluations the checks found to nest and end at one position only after they relied on
// the call.
func nestsLate(callee, caller string) string {
	return `|^the evaluations of the call of (?:` + callee + `) at [0-9]+:[0-9]+ in rule (?:` + caller + `) can nest without a bound and end at one position, `
}

// triageSmall is the triage table of csv, xml, cel, minilang, outline and calc.
func triageSmall() []TriageGroup {
	return []TriageGroup{
		// csv
		{Grammar: "csv", ID: "quoted-field", Witness: true, Rules: []string{"field"}, Pattern: `in rule field fails`,
			Shape:  "the quoted alternative of the atomic field rule fails after [^\"]* and the unquoted one starts again at the quote",
			Assign: "F97", Order: Linear, Families: []TriageFamily{
				{Name: "unterminated quote after n records", Build: func(n int) string { return rep("a,\"b\"\"c\",d\n", n) + "\"" + rep("x", n) }, Sizes: doubling(1000, 3), Lenient: true, Order: Linear},
				{Name: "quoted fields with doubled quotes", Build: list("", "\"a\"\"b\"\"c\",\"\"\"\"", ",", "\n"), Sizes: doubling(1000, 3), Order: Linear},
			}},

		// xml
		{Grammar: "xml", ID: "ws-option", Rules: []string{"main", "xml_decl", "doctype"}, Pattern: `contains the repetition \[ \\t\\r\\n\][*+] at`,
			Shape:  "an option that starts with whitespace against a continuation that starts with the same whitespace",
			Assign: "F97", Order: Linear, Families: []TriageFamily{
				{Name: "long whitespace in the XML declaration", Build: func(n int) string { return "<?xml version=\"1.0\"" + rep(" ", n) + "?>" + rep(" ", n) + "<r/>" }, Sizes: doubling(4000, 3), Order: Linear},
				{Name: "long whitespace in the doctype", Build: func(n int) string { return "<!DOCTYPE r" + rep(" ", n) + "><r/>" }, Sizes: doubling(4000, 3), Order: Linear},
			}},
		{Grammar: "xml", ID: "prolog-misc", Rules: []string{"misc_list"}, Pattern: `^an iteration of the repetition at [0-9]+:[0-9]+ in rule misc_list contains the repetition`,
			Shape:  "an iteration of the prolog list holds the scan of a comment, which the next list evaluates again at the same position through the memoized misc",
			Assign: "F100", Order: Linear, Families: []TriageFamily{
				{Name: "n processing instructions with long targets", Build: list("", "<?t%d"+rep("a", 50)+" d?>", "\n", "<r/>"), Sizes: doubling(500, 3), Order: Linear},
				{Name: "one target of length n", Build: func(n int) string { return "<?" + rep("a", n) + " d?><r/>" }, Sizes: doubling(4000, 3), Order: Linear},
				{Name: "n comments", Build: list("", "<!--"+rep("c", 50)+"-->", "\n", "<r/>"), Sizes: doubling(500, 3), Order: Linear},
				{Name: "one comment of length n", Build: func(n int) string { return "<!--" + rep("c", n) + "--><r/>" }, Sizes: doubling(4000, 3), Order: Linear},
			}},
		{Grammar: "xml", ID: "nested", Pattern: nestedPattern + nestsLate("cp", "choice"),
			Shape:  "units nested through the recursion of content_spec/cp and element/content, every call on it memoized",
			Assign: "F100", Order: Linear, Families: []TriageFamily{
				{Name: "nested elements", Build: nest("<e a=\"1\">", "t", "</e>"), Sizes: doubling(500, 3), Order: Linear},
				{Name: "nested content particles", Build: func(n int) string { return "<!DOCTYPE r [<!ELEMENT e " + nest("(", "a", ")")(n) + ">]><r/>" }, Sizes: doubling(500, 3), Order: Linear},
				{Name: "nested choices", Build: func(n int) string { return "<!DOCTYPE r [<!ELEMENT e " + nest("(b | ", "a", ")*")(n) + ">]><r/>" }, Sizes: doubling(500, 3), Order: Linear},
			}},
		{Grammar: "xml", ID: "name-scan", Witness: true, Rules: []string{"name", "s"}, Pattern: `examined`,
			Shape:  "a name or whitespace scan inside a token rule that a failed option or iteration gives back to a continuation that rescans it",
			Assign: "F97", Order: Linear, Families: []TriageFamily{
				{Name: "attribute list declarations", Build: list("<!DOCTYPE r [", "<!ATTLIST e%d a CDATA #IMPLIED bb CDATA #REQUIRED>", "\n", "]><r/>"), Sizes: doubling(300, 3), Order: Linear},
				{Name: "elements with long names and attributes", Build: func(n int) string {
					return list("<r>", "<e"+rep("n", 40)+" a=\"1\" b"+rep("c", 40)+"=\"2\" />", "", "</r>")(n)
				}, Sizes: doubling(300, 3), Order: Linear},
				{Name: "external entities and notations", Build: list("<!DOCTYPE r [", "<!ENTITY e%[1]d SYSTEM \"u\" NDATA n><!NOTATION n%[1]d PUBLIC \"p\">", "\n", "]><r/>"), Sizes: doubling(300, 3), Order: Linear},
			}},

		// outline
		{Grammar: "outline", ID: "blank-lines", Witness: true, Rules: []string{"blank"}, Pattern: `examined`,
			Shape:  "a blank-line scan [ \\t]* that a failed item gives back to the blank* after the block",
			Assign: "F97", Order: Linear, Families: []TriageFamily{
				{Name: "items separated by long blank lines", Build: list("", "item %d\n"+rep(" ", 40)+"\n", "", ""), Sizes: doubling(500, 3), Order: Linear},
				{Name: "deep children with trailing blanks", Build: func(n int) string {
					var s string
					for i := range stairDepth(n) {
						s += rep(" ", i) + "x\n" + rep(" ", i) + "\n"
					}
					return s
				}, Sizes: doubling(5000, 3), Order: Linear},
			}},
		{Grammar: "outline", ID: "nested", Pattern: nestedPattern,
			Shape:  "the item and block units nested through children, whose calls read the variable indent: a memoized call at a position is evaluated under the indentation of its enclosing items only",
			Assign: "F100", Order: Linear,
			Families: []TriageFamily{
				{Name: "staircase", Build: func(n int) string {
					var s string
					for i := range stairDepth(n) {
						s += rep(" ", i) + "x\n"
					}
					return s
				}, Sizes: doubling(5000, 3), Order: Linear},
			}},

		// cel
		{Grammar: "cel", ID: "number-tokens", Witness: true, Rules: []string{"int_plain", "double", "uint"}, Pattern: `examined`,
			Shape:  "the number alternatives int_plain / double / uint / int rescan the same digits inside atomic token rules",
			Assign: "F97", Order: Linear, Families: []TriageFamily{
				{Name: "long digit runs", Build: func(n int) string {
					return rep("1", n) + ".5 + " + rep("2", n) + "u + 0x" + rep("f", n) + " + " + rep("3", n) + "e5"
				}, Sizes: doubling(2000, 3), Order: Linear},
				{Name: "many doubles", Build: list("", "123.5e1", " + ", ""), Sizes: doubling(1000, 3), Order: Linear},
			}},
		{Grammar: "cel", ID: "string-tokens", Witness: true, Rules: []string{"raw_body"}, Pattern: `examined`,
			Shape:  "a raw triple-quoted body scanned by (!\"\\\"\\\"\\\"\" .)* inside the atomic string rule, before a later primary alternative",
			Assign: "F97", Order: Linear, Families: []TriageFamily{
				{Name: "long raw strings", Build: func(n int) string { return "r\"\"\"" + rep("a\"b", n) + "\"\"\" + r'''" + rep("b'c", n) + "'''" }, Sizes: doubling(2000, 3), Order: Linear},
			}},
		{Grammar: "cel", ID: "names", Witness: true, Rules: []string{"message", "name", "name_plain"}, Pattern: `examined`,
			Shape:  "the dotted-name lookahead of message and a backquoted name of a selector, rescanned by a later alternative or operator part",
			Assign: "F97", Order: Linear, Families: []TriageFamily{
				{Name: "long dotted names", Build: list("a", " . b", "", ""), Sizes: doubling(500, 3), Order: Linear},
				{Name: "backquoted selectors", Build: list("a", ".`"+rep("x ", 20)+"`", "", ""), Sizes: doubling(300, 3), Order: Linear},
			}},
		{Grammar: "cel", ID: "space", Witness: true, Rules: []string{"ws"}, Pattern: `examined`,
			Shape:  "the whitespace before a call or an operator part, rescanned by a later alternative, or by each enclosing ?: at the position where the nested operands end; the scan [ \\t\\r\\n\\u{c}]* repeats one character class, so no record answers it again",
			Assign: "O75", Order: Poly(2), Families: []TriageFamily{
				{Name: "space before a call", Build: list("", "f"+rep(" ", 30)+"(1)", " + ", ""), Sizes: doubling(300, 3), Order: Linear},
				{Name: "nested conditionals before long whitespace", Build: func(n int) string { return rep("a?b:", n) + "a" + rep(" ", n) }, Sizes: doubling(250, 3), Order: Poly(2)},
			}},
		{Grammar: "cel", ID: "nested", Pattern: nestedPattern,
			Shape:  "units of primary, unary, member and the list, map and field lists, nested through the recursion of expr, every call on it memoized",
			Assign: "F100", Order: Linear, Families: []TriageFamily{
				{Name: "nested parentheses", Build: nest("(", "1", ")"), Sizes: doubling(500, 3), Order: Linear},
				{Name: "nested lists", Build: nest("[1, ", "2", "]"), Sizes: doubling(500, 3), Order: Linear},
				{Name: "nested maps", Build: nest("{1: ", "2", "}"), Sizes: doubling(500, 3), Order: Linear},
				{Name: "nested calls", Build: nest("a.f(", "1", ")"), Sizes: doubling(500, 3), Order: Linear},
				{Name: "nested messages", Build: nest("A{x: ", "1", "}"), Sizes: doubling(500, 3), Order: Linear},
				{Name: "nested negations", Build: nest("-(!(", "1", "))"), Sizes: doubling(500, 3), Order: Linear},
			}},

		// minilang
		{Grammar: "minilang", ID: "recover", Rules: []string{"stmt", "if"}, Pattern: `.`,
			Shape:  "the #recover base of stmt, and else?, abandoned inside a block that the skip stops before, so the next statements parse the block again through the memoized stmt",
			Assign: "F100", Order: Linear, Families: []TriageFamily{
				{Name: "long junk lines", Build: list("", "x = a b c d e f g h;", "\n", ""), Sizes: doubling(500, 3), Lenient: true, Order: Linear},
				{Name: "else followed by junk", Build: list("", "if a { x = 1; } else junk junk;", "\n", ""), Sizes: doubling(500, 3), Lenient: true, Order: Linear},
				{Name: "nested blocks with junk", Build: nest("{ x x ", "y;", "}"), Sizes: doubling(200, 3), Lenient: true, Order: Linear},
			}},
		{Grammar: "minilang", ID: "nested-operators", Witness: true, Rules: []string{"ident"}, Pattern: `the operator parts that the enclosing operator parts of operation try again`,
			Shape:  "an operator part after an operand nested in prefix operators or the right operand of ?: fails after the skip, and each enclosing operator part tries the operator parts again at the same position",
			Assign: "F100", Order: Poly(2), Families: []TriageFamily{
				{Name: "nested negations before long whitespace", Build: func(n int) string { return rep("-", n) + "a" + rep(" ", n) + ";" }, Sizes: doubling(500, 3), Order: Linear, Generated: Poly(2)},
				{Name: "nested conditionals before long whitespace", Build: func(n int) string { return rep("a?b:", n) + "a" + rep(" ", n) + ";" }, Sizes: doubling(500, 3), Order: Linear, Generated: Poly(2)},
			}},
		{Grammar: "minilang", ID: "nested-operator-parts", Rules: []string{"operation"}, Pattern: `the operator parts that the enclosing operator parts of operation try again`,
			Shape:  "the operator parts after an operand, tried again by each enclosing prefix operator or ?: at the position where the nested operands end; the skip ws repeats a choice, so it is a run site whose records answer the walks again on the engine; a generated Go parser keeps no such records and scans the whitespace once per level, but records the text of a comment, a scan site",
			Assign: "F100", Order: Poly(2), Families: []TriageFamily{
				{Name: "nested negations before long whitespace", Build: func(n int) string { return rep("-", n) + "a" + rep(" ", n) + ";" }, Sizes: doubling(500, 3), Order: Linear, Generated: Poly(2)},
				// The text of the comment is a scan site, which a generated parser records too.
				{Name: "nested negations before a comment", Build: func(n int) string { return rep("!", n) + "a //" + rep("c", n) + "\n;" }, Sizes: doubling(500, 3), Order: Linear},
			}},

		// calc
		{Grammar: "calc", ID: "nested-operators", Rules: []string{"expr"}, Pattern: `the operator parts that the enclosing operator parts of expr try again`,
			Shape:  "the operator parts after an operand, tried again by each enclosing prefix operator or ^ at the position where the nested operands end; each try evaluates the skip ws again at that position, whose repetition of one character class is a scan site: its records answer the tries after the first walks, so the steps grow linearly, but the classifier does not exempt the skip of an Operators Rule",
			Assign: "O75", Order: Linear, Families: []TriageFamily{
				{Name: "nested negations before long whitespace", Build: func(n int) string { return rep("-", n) + "1" + rep(" ", n) }, Sizes: doubling(250, 3), Order: Linear},
				{Name: "nested powers before long whitespace", Build: func(n int) string { return rep("2^", n) + "1" + rep(" ", n) }, Sizes: doubling(250, 3), Order: Linear},
				{Name: "nested negations", Build: nest("-", "1", ""), Sizes: doubling(500, 3), Order: Linear},
			}},
	}
}
