package okta

// userName holds a human-friendly username that should be common between
// Teleport and Okta.
type userName string

// oktaAppID holds an Okta-generated ID string for an Okta application
type oktaAppID string

// oktaUserID holds an Okta-generated ID string for an Okta user. This is the
// persistent, unique identifier for a given Okta user, as the userName may
// change.
type oktaUserID string

// oktaGroupID holds the Okta-generated ID string for an Okta group. This is the
// persistent ID of a group, as the name and description can be changed.
type oktaGroupID string

// set maintains a collection of unique elements. Its implemented as the
// `classic map[T]struct{}` has the same underlying representation. This has the
// nice property of preserving the map-like reference semantics. You can even
// iterate over the set with `range`.
type set[T comparable] map[T]struct{}

// newSet constructs a set from an arbitrary collection of elements
func newSet[T comparable](elements ...T) set[T] {
	s := make(map[T]struct{}, len(elements))
	for _, e := range elements {
		s[e] = struct{}{}
	}
	return set[T](s)
}

// add expands the set to include the supplied element if not already included
func (s set[T]) add(element T) {
	s[element] = struct{}{}
}

// remove deletes an element from the set. Attempting to remove a non-existent
// element from the set is not considered an error.
func (s set[T]) remove(element T) {
	delete(s, element)
}

// has test for set membership
func (s set[T]) has(element T) bool {
	_, present := s[element]
	return present
}
