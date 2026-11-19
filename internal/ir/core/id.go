package core

// Number gives every node of the rules of m that has no ID one, in the order of the rules and of
// Walk, after LastID, and sets LastID. The builder of a module numbers it once it is built; a node
// reached twice keeps the ID it got first.
func Number(m *Module) {
	for _, r := range m.Rules {
		if r == nil {
			continue
		}
		Walk(r.Body, func(n Node) {
			if a := n.Attrs(); a.ID == 0 {
				a.ID = m.NewID()
			}
		})
	}
}

// NewID returns an ID that no node of m has, for a node that a pass adds and that takes the place
// of no other.
func (m *Module) NewID() NodeID {
	m.LastID++
	return m.LastID
}
