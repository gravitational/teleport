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
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/gravitational/teleport/lib/backend"
)

// newLease returns a non-nil [*backend.Lease] that's filled in with the details
// of the item (i.e. its key) if the item has an expiration time.
func newLease(i backend.Item) *backend.Lease {
	var lease backend.Lease
	if !i.Expires.IsZero() {
		lease.Key = i.Key
	}
	return &lease
}

// newRevision returns a random, non-null [pgtype.UUID] to use as a row
// revision.
func newRevision() pgtype.UUID {
	return pgtype.UUID{
		Bytes: uuid.New(),
		Valid: true,
	}
}

// idFromRevision derives a value usable as a [backend.Item]'s ID from a
// revision UUID.
func idFromRevision(revision uuid.UUID) int64 {
	u := binary.LittleEndian.Uint64(revision[:])
	u &= 0x7fff_ffff_ffff_ffff
	return int64(u)
}

// nonNilKey replaces an empty key with a non-nil one.
func nonNilKey(b backend.Key) []byte {
	if b == nil {
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
