// Package classretry is the generated parser of a grammar whose repetition tries a character class
// again from every "[" of a line that does not close it. It is the negative control of the scaling
// tests of the generated grammar-source parser.

package classretry

//go:generate go run ../../../../cmd/zp gen -g classretry.zp -module zeroparse.dev/zp/spec/language/internal/classretry -runtime import -o .
