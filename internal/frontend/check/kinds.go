package check

import (
	coreimpl "zeroparse.dev/zp/core/impl"
	"zeroparse.dev/zp/internal/frontend/types"
	"zeroparse.dev/zp/internal/runtime/nodes"
	"zeroparse.dev/zp/pkg/grammar/ast"
)

// Kinds are the run-time kinds of the values of a grammar's types.
type Kinds struct {
	// Types holds the node kind of each type that is not a named node type, by name: an alias has
	// the kind of the type it names.
	Types map[string]*coreimpl.Kind
	// Nodes holds the kinds of each named node type, also under the names of the aliases of it, which
	// share them.
	Nodes map[string]*nodes.NodeTypeKinds
}

// TypeKind returns the node kind of the type typ.
func (k *Kinds) TypeKind(typ string) *coreimpl.Kind {
	if t := k.Types[typ]; t != nil {
		return t
	}
	return coreimpl.KindOf(typ, "")
}

// Kinds returns the run-time kinds of the program's types, made on the first call that completes,
// which must come after the types are checked; poll, if not nil, is called for each type and may
// cancel the call with a panic.
func (p *Checker) Kinds(poll func()) *Kinds {
	p.kindsMu.Lock()
	defer p.kindsMu.Unlock()
	if p.kinds == nil {
		k := &Kinds{Types: map[string]*coreimpl.Kind{}, Nodes: map[string]*nodes.NodeTypeKinds{}}
		for name, spec := range p.TypeDecls() {
			if poll != nil {
				poll()
			}
			if n, ok := p.NodeTypeOf(name); ok {
				if k.Nodes[n.Name] == nil {
					k.Nodes[n.Name] = nodes.NewNodeTypeKinds(n.Name)
				}
				k.Nodes[name] = k.Nodes[n.Name]
				continue
			}
			canonical := name
			if _, alias := spec.(*ast.TypeAlias); alias {
				if resolved, concrete := p.ConcreteType(&ast.TypeRef{Name: name}); concrete != nil {
					canonical = resolved
				}
			}
			k.Types[name] = coreimpl.KindOf(canonical, "")
		}
		p.kinds = k
	}
	return p.kinds
}

// ResolveCheck resolves the type of a cast, as a value type. It returns nil for _, which every value
// fits.
func (p *Checker) ResolveCheck(t ast.TypeExpr) types.Ty {
	if t == nil {
		return nil
	}
	if r := (&TypeResolver{Prog: p}).Resolve(t); types.NewFieldType(r) != nil {
		return r
	}
	return nil
}

// CastType returns the type the cast t checks its value against, or nil if every value fits it. The
// type of each cast is resolved once.
func (p *Checker) CastType(t *ast.Cast) *types.FieldType {
	if v, ok := p.castTypes.Load(t); ok {
		return v.(*types.FieldType)
	}
	var ft *types.FieldType
	if r := p.ResolveCheck(t.Type); r != nil {
		ft = types.NewFieldType(r)
	}
	v, _ := p.castTypes.LoadOrStore(t, ft)
	return v.(*types.FieldType)
}
