package gogen

// ordinaryHelperName reports whether the library has an Ordinary form of the method name, which
// a parser without budget controls and Documents calls instead (the library writes them by hand).
func ordinaryHelperName(name string) bool {
	switch name {
	case "NewNode", "NewTerminal", "NewBranch", "NewMatch", "Expect", "Peek", "MatchLiteral", "MatchLiteralFold", "MatchAny",
		"AtEnd", "AtLineStart", "AtEndBatch", "AtLineStartBatch", "AtLineEnd", "SpendRead":
		return true
	}
	return false
}
