package compile

import "zeroparse.dev/zp/grammar"

// SurelyConsumed returns the characters of chars on which the classifier of g proves that a call of
// each rule surely consumes.
func SurelyConsumed(g *grammar.Grammar, chars string) (map[string]string, error) {
	k, _, err := prepareClassifier(g, "", nil)
	if err != nil {
		return nil, err
	}
	k.collect()
	k.computeBoundedScans()
	out := map[string]string{}
	for _, r := range k.rules {
		s := k.sure(&grammar.Ref{Name: r.Name})
		for _, c := range chars {
			if s.intersects(charOf(c)) {
				out[r.Name] += string(c)
			}
		}
	}
	return out, nil
}
