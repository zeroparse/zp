package print

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

// tracedHelperName reports whether the library has a Traced form of the read name, which records
// the examined end of a stream read, for the traced Node bodies (the library writes them by hand).
func tracedHelperName(name string) bool {
	switch name {
	case "Peek", "MatchLiteral", "MatchLiteralFold", "AtLineEnd":
		return true
	}
	return false
}
