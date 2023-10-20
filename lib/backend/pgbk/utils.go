// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pgbk

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
// in second position.
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
