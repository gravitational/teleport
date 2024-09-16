package set

// Set maintains a collection of unique elements. Its implemented as the
// `classic map[T]struct{}` has the same underlying representation. This has the
// nice property of preserving the map-like reference semantics. You can even
// iterate over the set with `range`.
type Set[T comparable] map[T]struct{}

// New constructs a set from an arbitrary collection of elements
func New[T comparable](elements ...T) Set[T] {
	s := make(map[T]struct{}, len(elements))
	for _, e := range elements {
		s[e] = struct{}{}
	}
	return s
}

// Add expands the set to include the supplied element if not already included
func (s Set[T]) Add(element T) {
	s[element] = struct{}{}
}

// Remove deletes an element from the set. Attempting to remove a non-existent
// element from the set is not considered an error.
func (s Set[T]) Remove(element T) {
	delete(s, element)
}

// Has test for set membership
func (s Set[T]) Has(element T) bool {
	_, present := s[element]
	return present
}
