package archtest

import (
	"fmt"
	"slices"
	"strings"
)

// A violation is an import edge that breaks a rule.
type violation struct {
	rule     string   // "1", "1a" or "2"
	from, to string   // the edge: module-relative paths, or the name of a generated parser
	why      []string // the clauses it breaks
	chain    []string // the shortest import chain that shows it
	builds   []string // the builds that have it
}

// key identifies the violation in the allowlist.
func (v *violation) key() string {
	return fmt.Sprintf("rule %s: %s -> %s", v.rule, v.from, v.to)
}

// A generatedParser is a Go parser that the test generated, with the imports of its files.
type generatedParser struct {
	name    string // how the report names it
	module  string // its import path; imports below it are its own packages
	imports []string
}

// A report is the outcome of checking the graphs.
type report struct {
	errs       []string // packages that the layout does not place, and rows that place none
	violations map[string]*violation
}

// buildName names the build of a graph in a report.
func buildName(tags string) string {
	if tags == "" {
		return "default"
	}
	return tags
}

// check checks the rules on the graphs and the generated parsers.
func check(graphs []*graph, gens []generatedParser) *report {
	r := &report{violations: map[string]*violation{}}
	used := make([]bool, len(layout))
	places := map[string]placed{}
	placeErr := map[string]bool{}
	// target places the module package p, recording a package that no row places.
	target := func(p string) (placed, bool) {
		if pl, ok := places[p]; ok {
			return pl, true
		}
		pl, err := placeOf(rel(p))
		if err != nil {
			if !placeErr[p] {
				placeErr[p] = true
				r.errs = append(r.errs, err.Error())
			}
			return placed{}, false
		}
		used[pl.row] = true
		places[p] = pl
		return pl, true
	}
	for _, g := range graphs {
		var modPkgs []string
		for p := range g.pkgs {
			if inModule(p) {
				modPkgs = append(modPkgs, p)
			}
		}
		slices.Sort(modPkgs)
		var roots []string // the public runtime-side packages, where chains of rule 1 start
		for _, p := range modPkgs {
			if pl, ok := target(p); ok && publicRuntime(pl.target) {
				roots = append(roots, p)
			}
		}
		add := func(rule, from, to string, why string, chain []string) {
			v := &violation{rule: rule, from: from, to: to}
			if old, ok := r.violations[v.key()]; ok {
				v = old
			} else {
				v.chain = chain
				r.violations[v.key()] = v
			}
			if !slices.Contains(v.why, why) {
				v.why = append(v.why, why)
			}
			if b := buildName(g.tags); !slices.Contains(v.builds, b) {
				v.builds = append(v.builds, b)
			}
		}
		runtimeEdge := func(a, b string) bool {
			if !inModule(b) {
				return false
			}
			pa, oka := target(a)
			pb, okb := target(b)
			return oka && okb && runtimeSide(pa.target) && runtimeSide(pb.target)
		}
		// runtimeChain returns the shortest chain from a public runtime-side package to the
		// runtime-side package a, or a alone when no public one reaches it.
		runtimeChain := func(a string) []string {
			if chain := g.shortestChain(roots, a, runtimeEdge); chain != nil {
				return relAll(chain)
			}
			return []string{rel(a)}
		}
		// engineChains adds a rule 1a violation for each package that a generated parser must not
		// link and that the module packages imps reach; name is the generated parser, and head
		// starts each chain before imps.
		engineChains := func(name string, head, imps []string) {
			for _, p := range modPkgs {
				pl, ok := target(p)
				if !ok || !engineSide(pl.target) {
					continue
				}
				// Stop at the first engine-side package on each chain, so that each forbidden
				// package is reported where the generated parser first reaches the engine.
				chain := g.shortestChain(imps, p, func(a, b string) bool {
					if !inModule(b) {
						return false
					}
					pa, _ := target(a)
					return !engineSide(pa.target)
				})
				if chain != nil {
					add("1a", name, rel(p), "a generated parser links neither pkg/runtime/vm nor internal/runtime/**", append(slices.Clone(head), relAll(chain)...))
				}
			}
		}
		for _, a := range modPkgs {
			pa, ok := target(a)
			if !ok {
				continue
			}
			importsGolib := false
			for _, b := range g.pkgs[a].Imports {
				if !inModule(b) {
					if g.pkgs[b] != nil && !g.pkgs[b].Standard {
						if runtimeSide(pa.target) {
							add("1", rel(a), b, "the runtime side imports no third-party package", append(runtimeChain(a), b))
						}
						if pa.generated {
							add("1a", rel(a), b, "a generated parser imports no third-party package", []string{rel(a), b})
						}
					}
					continue
				}
				pb, ok := target(b)
				if !ok {
					continue
				}
				edge := []string{rel(a), rel(b)}
				// Rule 1.
				if runtimeSide(pa.target) && !runtimeSide(pb.target) {
					add("1", rel(a), rel(b), fmt.Sprintf("the runtime side (%s) imports only the runtime side, not %s", pa.target, pb.target), append(runtimeChain(a), rel(b)))
				}
				// Rule 1a: the library and the generated parsers.
				if pb.target == golib {
					importsGolib = true
				}
				if pa.target == golib && !libraryImport(pb.target) {
					add("1a", rel(a), rel(b), fmt.Sprintf("%s imports only pkg/runtime and its input, value, value/impl, instrument and schema packages, not %s", golib, pb.target), edge)
				}
				if pa.generated && pb.target != golib && !libraryImport(pb.target) && !pb.generated {
					add("1a", rel(a), rel(b), fmt.Sprintf("a generated parser imports only %s, the packages it may import and generated packages, not %s", golib, pb.target), edge)
				}
				// Rule 2.
				for _, why := range layer2(pa.target, pb.target) {
					add("2", rel(a), rel(b), fmt.Sprintf("%s (%s -> %s)", why, pa.target, pb.target), edge)
				}
			}
			if importsGolib && !pa.generated && !runtimeSide(pa.target) {
				if !placeErr[a] {
					placeErr[a] = true
					r.errs = append(r.errs, fmt.Sprintf("%s imports the library of generated parsers but its layout row does not mark it generated", rel(a)))
				}
			}
			if pa.generated {
				engineChains(rel(a), nil, []string{a})
			}
		}
		for _, gen := range gens {
			var mod []string
			for _, b := range gen.imports {
				switch {
				case b == gen.module || strings.HasPrefix(b, gen.module+"/"):
				case inModule(b):
					mod = append(mod, b)
					pb, ok := target(b)
					if ok && pb.target != golib && !libraryImport(pb.target) {
						add("1a", gen.name, rel(b), fmt.Sprintf("a generated parser imports only %s and the packages it may import, not %s", golib, pb.target), []string{gen.name, rel(b)})
					}
				case !strings.Contains(strings.Split(b, "/")[0], "."):
					// The standard library.
				default:
					add("1a", gen.name, b, "a generated parser imports no third-party package", []string{gen.name, b})
				}
			}
			engineChains(gen.name, []string{gen.name}, mod)
		}
	}
	for i, used := range used {
		if !used {
			r.errs = append(r.errs, fmt.Sprintf("layout row %q places no package of any build; remove it", layout[i].pkg))
		}
	}
	return r
}

// relAll returns the module-relative paths of the module packages ps.
func relAll(ps []string) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		if inModule(p) {
			out[i] = rel(p)
		} else {
			out[i] = p
		}
	}
	return out
}

// compare compares the violations with the allowlist and returns one message per difference.
func (r *report) compare(allow []string, nbuilds int) []string {
	msgs := slices.Clone(r.errs)
	listed := map[string]bool{}
	for _, k := range allow {
		if listed[k] {
			msgs = append(msgs, fmt.Sprintf("allowlist lists %q twice", k))
		}
		listed[k] = true
		if r.violations[k] == nil {
			msgs = append(msgs, fmt.Sprintf("%s: no build has this edge any more; remove it from the allowlist", k))
		}
	}
	keys := make([]string, 0, len(r.violations))
	for k := range r.violations {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		if listed[k] {
			continue
		}
		v := r.violations[k]
		builds := "every build"
		if len(v.builds) < nbuilds {
			builds = strings.Join(v.builds, ", ")
		}
		msgs = append(msgs, fmt.Sprintf("%s\n\tbreaks: %s\n\tchain: %s\n\tbuilds: %s",
			k, strings.Join(v.why, "; "), strings.Join(v.chain, " -> "), builds))
	}
	return msgs
}
