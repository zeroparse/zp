package program

// Scope holds the capture slots of a frame: the names by slot, and the slot of each name.
type Scope struct {
	Names []string
	Slots map[string]int
}

// NewScope returns an empty scope.
func NewScope() *Scope { return &Scope{Slots: map[string]int{}} }

// ScopeOf returns the scope whose slots hold names, in order.
func ScopeOf(names []string) *Scope {
	s := &Scope{Names: names, Slots: make(map[string]int, len(names))}
	for i, n := range names {
		s.Slots[n] = i
	}
	return s
}

// Has reports whether the scope has the capture name.
func (s *Scope) Has(name string) bool { _, ok := s.Slots[name]; return ok }

// Slot returns the slot of the capture name, adding one if the scope has none.
func (s *Scope) Slot(name string) int {
	if i, ok := s.Slots[name]; ok {
		return i
	}
	s.Slots[name] = len(s.Names)
	s.Names = append(s.Names, name)
	return s.Slots[name]
}
