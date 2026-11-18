package pass

import (
	"context"
	"fmt"
	"slices"
	"sync"
)

// Analyses.
//
// An analysis computes data about a unit that a pass, a check or a tool reads, without writing it
// into the unit. An AnalysisManager computes each analysis of its unit when it is first asked for
// and keeps the result, so that the passes and tools that read the same data share one
// computation. A pass that changes the unit says which analyses it preserves; the manager discards
// every other one, and every analysis that used a discarded one, directly or not. Nothing is
// preserved unless a pass says so: a pass that forgets to declare a preserved analysis costs a
// recomputation, never a stale result.

// PreservedSet is the set of analyses that a change of a unit leaves valid. The zero value
// preserves none.
type PreservedSet struct {
	all  bool
	keys map[string]bool
}

// PreserveAll preserves every analysis: the change did not alter the unit.
func PreserveAll() PreservedSet { return PreservedSet{all: true} }

// PreserveNone preserves no analysis, as the zero value does.
func PreserveNone() PreservedSet { return PreservedSet{} }

// Preserve preserves the analyses of the given names.
func Preserve(names ...string) PreservedSet {
	p := PreservedSet{keys: map[string]bool{}}
	for _, n := range names {
		p.keys[n] = true
	}
	return p
}

// Has reports whether the analysis of the given name is preserved.
func (p PreservedSet) Has(name string) bool { return p.all || p.keys[name] }

// AnalysisKey names an analysis of units of type T with results of type R, and says how to compute
// it. Run may ask the manager for other analyses of the same unit; the manager records them as its
// dependencies.
type AnalysisKey[T, R any] struct {
	name string
	run  func(ir T, am *AnalysisManager[T]) (R, error)
}

// Name returns the name of the analysis.
func (k AnalysisKey[T, R]) Name() string { return k.name }

var (
	keysMu sync.Mutex
	keys   = map[string]bool{}
)

// NewAnalysisKey declares the analysis of the given name, computed by run. Names are unique across
// the program, so that two analyses with one name and different results cannot take each other's
// cached results: a second declaration of a name panics. Keys are declared once, in package
// variables.
func NewAnalysisKey[T, R any](name string, run func(ir T, am *AnalysisManager[T]) (R, error)) AnalysisKey[T, R] {
	keysMu.Lock()
	defer keysMu.Unlock()
	if name == "" || run == nil {
		panic("pass: an analysis key has no name or no computation")
	}
	if keys[name] {
		panic(fmt.Sprintf("pass: the analysis %q is declared twice", name))
	}
	keys[name] = true
	return AnalysisKey[T, R]{name: name, run: run}
}

// AnalysisManager caches the analyses of one unit of type T. It is not safe for concurrent use.
type AnalysisManager[T any] struct {
	ctx context.Context
	// ir is the unit the cached results are of; a request for another unit discards them.
	ir    T
	bound bool
	cache map[string]any
	// deps holds, for each cached analysis, the analyses it asked for while it ran.
	deps    map[string]map[string]bool
	running []string
	// computed counts the computations of each analysis.
	computed map[string]int
}

// NewAnalysisManager returns a manager with no cached analysis. ctx, which may be nil, is the
// context of the work that the analyses are part of (Context).
func NewAnalysisManager[T any](ctx context.Context) *AnalysisManager[T] {
	return &AnalysisManager[T]{ctx: ctx, cache: map[string]any{}, deps: map[string]map[string]bool{},
		computed: map[string]int{}}
}

// Context returns the context of the work, which an analysis polls for cancellation, or nil.
func (am *AnalysisManager[T]) Context() context.Context { return am.ctx }

// Get returns the result of the analysis key of ir, computing it if it is not cached. An analysis
// that fails returns its error, and nothing is cached; an analysis that asks for itself, directly
// or not, fails. A request for a unit other than the one the cache is of discards the cache first,
// so a cached result is always of the unit asked about.
func Get[T, R any](am *AnalysisManager[T], ir T, key AnalysisKey[T, R]) (R, error) {
	var zero R
	if key.run == nil {
		return zero, fmt.Errorf("analysis %q is not declared with NewAnalysisKey", key.name)
	}
	v, err := getNamed(am, ir, key.name, func(ir T, am *AnalysisManager[T]) (any, error) { return key.run(ir, am) })
	if err != nil {
		return zero, err
	}
	return v.(R), nil
}

// getNamed is Get of the analysis of the given name, computed by run.
func getNamed[T any](am *AnalysisManager[T], ir T, name string, run func(T, *AnalysisManager[T]) (any, error)) (any, error) {
	if !am.bound || !sameUnit(am.ir, ir) {
		if len(am.running) > 0 {
			return nil, fmt.Errorf("analysis %s asks for an analysis of another unit", am.running[len(am.running)-1])
		}
		am.ir, am.bound = ir, true
		clear(am.cache)
		clear(am.deps)
	}
	if n := len(am.running); n > 0 {
		caller := am.running[n-1]
		if am.deps[caller] == nil {
			am.deps[caller] = map[string]bool{}
		}
		am.deps[caller][name] = true
	}
	if v, ok := am.cache[name]; ok {
		return v, nil
	}
	if slices.Contains(am.running, name) {
		return nil, fmt.Errorf("analysis %s depends on itself", name)
	}
	am.running = append(am.running, name)
	am.computed[name]++
	computations.add(name)
	res, err := runAnalysis(am, ir, name, run)
	am.running = am.running[:len(am.running)-1]
	if err != nil {
		delete(am.deps, name)
		return nil, fmt.Errorf("analysis %s: %w", name, err)
	}
	am.cache[name] = res
	return res, nil
}

// runAnalysis runs the analysis, leaving the stack of running analyses as it found it if the
// analysis panics, as a cancellation does.
func runAnalysis[T any](am *AnalysisManager[T], ir T, name string, run func(T, *AnalysisManager[T]) (any, error)) (any, error) {
	n := len(am.running)
	defer func() {
		if r := recover(); r != nil {
			am.running = am.running[:n-1]
			delete(am.deps, name)
			panic(r)
		}
	}()
	return run(ir, am)
}

// sameUnit reports whether a and b are the same unit: equal, for a comparable T such as a pointer.
func sameUnit[T any](a, b T) bool {
	defer func() { _ = recover() }()
	return any(a) == any(b)
}

// Cached reports whether the result of the analysis of the given name is cached.
func (am *AnalysisManager[T]) Cached(name string) bool {
	_, ok := am.cache[name]
	return ok
}

// Computations returns how many times the manager has computed the analysis of the given name.
func (am *AnalysisManager[T]) Computations(name string) int { return am.computed[name] }

// Invalidate discards every cached analysis that p does not preserve, and every analysis that
// used a discarded one, directly or not.
func (am *AnalysisManager[T]) Invalidate(p PreservedSet) {
	if p.all {
		return
	}
	stale := map[string]bool{}
	for name := range am.cache {
		if !p.Has(name) {
			stale[name] = true
		}
	}
	for changed := true; changed; {
		changed = false
		for name := range am.cache {
			if stale[name] {
				continue
			}
			for dep := range am.deps[name] {
				if stale[dep] || !am.Cached(dep) {
					stale[name], changed = true, true
					break
				}
			}
		}
	}
	for name := range stale {
		delete(am.cache, name)
		delete(am.deps, name)
	}
}

// computations counts the computations of each analysis in the program, by every manager, so that
// a measurement can tell how often an analysis is computed for a piece of work (Computations).
var computations = counter{n: map[string]int{}}

type counter struct {
	mu sync.Mutex
	n  map[string]int
}

func (c *counter) add(name string) {
	c.mu.Lock()
	c.n[name]++
	c.mu.Unlock()
}

// Computations returns how many times the analysis of the given name has been computed in the
// program, by every manager. A measurement takes the difference over the work it measures.
func Computations(name string) int {
	computations.mu.Lock()
	defer computations.mu.Unlock()
	return computations.n[name]
}
