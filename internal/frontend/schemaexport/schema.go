// Package schemaexport exports the output schema of a checked program as a schema.Output.
package schemaexport

import (
	"encoding/json"
	"fmt"
	"slices"
	"sort"

	"zeroparse.dev/zp/internal/frontend/astfacts"
	"zeroparse.dev/zp/internal/frontend/check"
	"zeroparse.dev/zp/internal/frontend/types"
	"zeroparse.dev/zp/internal/runtime/program"
	"zeroparse.dev/zp/pkg/grammar/ast"
	"zeroparse.dev/zp/pkg/runtime/schema"
)

// ErrUnavailable reports missing checked grammar metadata: it is program.ErrSchemaUnavailable,
// which the runtime returns for a program that has no schema.
var ErrUnavailable = program.ErrSchemaUnavailable

// Source is the checked grammar that a schema is exported from.
type Source struct {
	// Model is the grammar's semantic model, and Checker the checker of its types; its Typed is nil
	// if the types were not checked, and then the grammar has no schema.
	Model   *check.Model
	Checker *check.Checker
	// SourceInfo reports that the values record source information.
	SourceInfo bool
}

// Export exports the schema of the checked grammar p for the start rule start: the rule, the rules
// it calls or whose values its values contain, and the struct types they refer to; or every rule
// and type with allRules. entry.SchemaSource checks the start rule first.
func Export(p Source, start string, allRules bool) (*schema.Output, error) {
	if p.Model == nil || p.Model.Grammar == nil || p.Checker == nil || p.Checker.Typed == nil {
		return nil, ErrUnavailable
	}
	g, c := p.Model.Grammar, p.Checker
	if c.Rule(start).Def == nil {
		return nil, fmt.Errorf("unknown schema start rule %q", start)
	}
	// Without all rules, the export is the start rule's reachable part, which is stored per start rule.
	if !allRules && !slices.Contains(p.Model.StartRules, start) {
		return nil, fmt.Errorf("unknown schema start rule %q", start)
	}
	values := c.ValueTypes()
	if values == nil {
		return nil, ErrUnavailable
	}
	all := map[string]schema.Type{}
	for _, td := range g.Types() {
		spec, ok := td.(*ast.TypeDef)
		if !ok {
			continue // aliases name other types
		}
		id := "declared/" + td.DeclName()
		if spec.Struct() == nil {
			// A named node type has the shape of its underlying node type.
			n, ok := c.NodeTypeOf(td.DeclName())
			if !ok {
				return nil, fmt.Errorf("schema type %q: unresolved node type", td.DeclName())
			}
			shape, err := shapeOf(n.Underlying())
			if err != nil {
				return nil, fmt.Errorf("schema type %q: %w", td.DeclName(), err)
			}
			all[id] = schema.Type{ID: id, Name: td.DeclName(), Origin: "declared", Shape: shape}
			continue
		}
		shape := schema.Shape{Kind: "struct"}
		for _, f := range check.DefFields(spec) {
			fieldShape, err := declared(c, f.Type)
			if err != nil {
				return nil, fmt.Errorf("schema type %q field %q: %w", td.DeclName(), f.Name, err)
			}
			shape.Fields = append(shape.Fields, schema.Field{Name: f.Name, Shape: fieldShape})
		}
		sort.Slice(shape.Fields, func(i, j int) bool { return shape.Fields[i].Name < shape.Fields[j].Name })
		all[id] = schema.Type{ID: id, Name: td.DeclName(), Origin: "declared", Shape: shape}
	}
	s := &schema.Output{Format: schema.Format, Version: schema.Version, Start: start, PositionUnitIndependent: true, SourceInfo: p.SourceInfo, Types: []schema.Type{}, Rules: []schema.Rule{}}
	selected := map[string]bool{}
	noType := map[string]bool{}
	wanted := map[string]bool{}
	var todo []string
	var collect func(schema.Shape)
	collect = func(shape schema.Shape) {
		if shape.Ref != "" && !wanted[shape.Ref] {
			wanted[shape.Ref] = true
			if t, ok := all[shape.Ref]; ok {
				collect(t.Shape)
			} else if name, ok := cutPrefix(shape.Ref, "inferred/"); ok {
				todo = append(todo, name)
			}
		}
		for _, sub := range []*schema.Shape{shape.Element, shape.Value} {
			if sub != nil {
				collect(*sub)
			}
		}
		for _, list := range [][]schema.Shape{shape.Elements, shape.Alternatives} {
			for _, sub := range list {
				collect(sub)
			}
		}
		for _, f := range shape.Fields {
			collect(f.Shape)
		}
		for _, c := range shape.Captures {
			collect(c.Shape)
		}
	}
	todo = append(todo, start)
	if allRules {
		for _, r := range g.Rules() {
			todo = append(todo, r.Name)
		}
	}
	for len(todo) > 0 {
		name := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		if selected[name] {
			continue
		}
		selected[name] = true
		rd := c.Rule(name).Def
		if rd == nil {
			return nil, fmt.Errorf("unresolved schema rule %q", name)
		}
		astfacts.WalkExpr(rd.Expr, func(x ast.Expr) {
			if r, ok := x.(*ast.Ref); ok && !selected[r.Name] {
				todo = append(todo, r.Name)
			}
		})
		t, ok := values[name]
		if _, splice := c.SpliceBody(name); splice && !ok {
			noType[name] = true // a rule that makes no node has a type only where another type refers to it
			continue
		}
		if !ok || t == nil {
			return nil, ErrUnavailable
		}
		shape, err := shapeOf(t)
		if err != nil {
			return nil, fmt.Errorf("schema rule %q: %w", name, err)
		}
		id := "inferred/" + name
		all[id] = schema.Type{ID: id, Name: name, Origin: "inferred", Shape: shape}
		wanted[id] = true
		collect(shape)
	}
	for _, rd := range g.Rules() {
		if !selected[rd.Name] || noType[rd.Name] {
			continue
		}
		origin := "inferred"
		if rd.Type != nil {
			origin = "declared"
		}
		s.Rules = append(s.Rules, schema.Rule{Name: rd.Name, Origin: origin, Result: schema.Shape{Kind: "ref", Ref: "inferred/" + rd.Name}})
	}
	if allRules {
		for id := range all {
			wanted[id] = true
		}
	}
	for id := range wanted {
		t, ok := all[id]
		if !ok {
			return nil, fmt.Errorf("unresolved schema reference %q", id)
		}
		s.Types = append(s.Types, t)
	}
	sort.Slice(s.Types, func(i, j int) bool { return s.Types[i].ID < s.Types[j].ID })
	sort.Slice(s.Rules, func(i, j int) bool { return s.Rules[i].Name < s.Rules[j].Name })
	return s, nil
}

func cutPrefix(s, prefix string) (string, bool) {
	if len(s) >= len(prefix) && s[:len(prefix)] == prefix {
		return s[len(prefix):], true
	}
	return "", false
}

// declared describes the type a field declares.
func declared(c *check.Checker, t ast.TypeExpr) (schema.Shape, error) {
	k := &check.TypeResolver{Prog: c}
	return shapeOf(k.Resolve(t))
}

// shapeOf describes a type of the value model.
func shapeOf(t types.Ty) (schema.Shape, error) {
	switch t := t.(type) {
	case types.BasicTy:
		switch t {
		case types.TyBottom:
			return schema.Shape{Kind: "join"}, nil // no values
		case types.TyInt, types.TyString, types.TyBool:
			return schema.Shape{Kind: "scalar", Scalar: string(t)}, nil
		case types.TyNil:
			return schema.Shape{Kind: "nil"}, nil
		case types.TyErr:
			return schema.Shape{Kind: "error"}, nil
		case types.TyTop, types.TyAny:
			return schema.Shape{Kind: "top"}, nil
		}
		return schema.Shape{}, fmt.Errorf("unsupported schema type %q", t)
	case types.NamedTy:
		if t == types.TyMissing {
			return schema.Shape{Kind: "missing"}, nil
		}
		if t.Kind != 's' {
			v := types.FromLegacy(t)
			if n, ok := v.(types.NamedTy); !ok || n.Kind == 's' {
				return shapeOf(v)
			}
			return schema.Shape{}, fmt.Errorf("unresolved schema type %s", t.Name)
		}
		return schema.Shape{Kind: "ref", Ref: "declared/" + t.Name}, nil
	case types.RuleTy:
		return schema.Shape{Kind: "ref", Ref: "inferred/" + t.Name}, nil
	case types.NodeTy:
		if t.Name != "" {
			return schema.Shape{Kind: "ref", Ref: "declared/" + t.Name}, nil
		}
		v, err := shapeOf(t.Value)
		if err != nil {
			return schema.Shape{}, err
		}
		s := schema.Shape{Kind: "node", Value: &v}
		// A name captured in several places (an alternative, or a lookahead beside the tuple)
		// has one entry: its locations are the union of the places, and its type is the join of
		// the types of the places, joined once so that the result is flat and sorted.
		type capGroup struct {
			name  string
			paths [][]int
			side  bool
			types []types.Ty
		}
		var groups []*capGroup
		index := map[string]*capGroup{}
		for _, c := range t.Caps {
			g, ok := index[c.Name]
			if !ok {
				g = &capGroup{name: c.Name}
				index[c.Name] = g
				groups = append(groups, g)
			}
			var paths [][]int
			for _, path := range c.Paths {
				paths = append(paths, append([]int{}, path...))
			}
			g.paths = mergeCapturePaths(g.paths, paths)
			g.side = g.side || c.Side
			g.types = append(g.types, c.T)
		}
		sort.Slice(groups, func(i, j int) bool { return groups[i].name < groups[j].name })
		for _, g := range groups {
			cs, err := shapeOf(types.Vjoin(g.types...))
			if err != nil {
				return schema.Shape{}, fmt.Errorf("capture %s: %w", g.name, err)
			}
			s.Captures = append(s.Captures, schema.Capture{Name: g.name, Paths: g.paths, Side: g.side, Shape: cs})
		}
		return s, nil
	case types.TupleTy:
		s := schema.Shape{Kind: "tuple"}
		for _, e := range t.Elems {
			es, err := shapeOf(e)
			if err != nil {
				return schema.Shape{}, err
			}
			s.Elements = append(s.Elements, es)
		}
		return s, nil
	case types.ListTy:
		e, err := shapeOf(t.Elem)
		return schema.Shape{Kind: "list", Element: &e}, err
	case types.OptTy:
		e, err := shapeOf(t.Elem)
		return schema.Shape{Kind: "optional", Element: &e}, err
	case types.JoinTy:
		s := schema.Shape{Kind: "join"}
		for _, a := range t.Alts {
			v, err := shapeOf(a)
			if err != nil {
				return s, err
			}
			s.Alternatives = append(s.Alternatives, v)
		}
		sortSchemaAlternatives(s.Alternatives)
		return s, nil
	case types.RecordTy:
		return shapeOf(types.FromLegacy(t))
	}
	return schema.Shape{}, fmt.Errorf("unresolved schema type %T", t)
}

func sortSchemaAlternatives(alts []schema.Shape) {
	sort.Slice(alts, func(i, j int) bool {
		a, _ := json.Marshal(alts[i])
		b, _ := json.Marshal(alts[j])
		return string(a) < string(b)
	})
}

// mergeCapturePaths returns the sorted union of two sets of capture paths, without duplicates.
func mergeCapturePaths(a, b [][]int) [][]int {
	out := append(append([][]int{}, a...), b...)
	sort.Slice(out, func(i, j int) bool { return slices.Compare(out[i], out[j]) < 0 })
	return slices.CompactFunc(out, func(x, y []int) bool { return slices.Equal(x, y) })
}
