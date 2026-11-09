// Package desugar holds the first step of the front end: the rewrites of a parsed grammar into
// the smaller language that the later steps check and analyze. It lowers inline actions to
// hidden rules, normalizes attributes and predicate blocks, resolves the start rules, takes the
// first declaration of each rule name, and builds the default-value grammar. Each rewrite leaves
// its input unchanged and shares the declarations it does not change.
package desugar
