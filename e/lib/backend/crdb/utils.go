package crdb

import (
	"github.com/google/uuid"

	"github.com/gravitational/teleport/lib/backend"
)

// revision is transparently converted to and from Postgres UUIDs.
type revision = [16]byte

// newRevision returns a new random revision.
func newRevision() revision {
	return revision(uuid.New())
}

// revisionToString converts a revision to its string form, usable in
// [backend.Item].
func revisionToString(r revision) string {
	return uuid.UUID(r).String()
}

// revisionFromString converts a revision from its string form, returning false
// in second position if string isn't a valid UUID.
func revisionFromString(s string) (r revision, ok bool) {
	u, err := uuid.Parse(s)
	if err != nil {
		return revision{}, false
	}
	return u, true
}

// nonNilKey replaces an empty key with a non-nil one.
func nonNilKey(b backend.Key) []byte {
	if b.IsZero() {
		return []byte{}
	}

	return []byte(b.String())
}

// nonNil replaces a nil slice with an empty, non-nil one.
func nonNil(b []byte) []byte {
	if b == nil {
		return []byte{}
	}
	return b
}
