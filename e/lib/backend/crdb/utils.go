package crdb

import (
	"encoding/binary"

	"github.com/google/uuid"
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

// idFromRevision derives a value usable as a [backend.Item]'s ID from a
// revision UUID.
func idFromRevision(revision revision) int64 {
	u := binary.LittleEndian.Uint64(revision[:])
	u &= 0x7fff_ffff_ffff_ffff
	return int64(u)
}

// nonNil replaces a nil slice with an empty, non-nil one.
func nonNil(b []byte) []byte {
	if b == nil {
		return []byte{}
	}
	return b
}
