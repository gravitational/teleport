/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package pgevents

import (
	"encoding/base64"
	"encoding/binary"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
)

type paginationKey struct {
	time     time.Time
	index    int64
	id       uuid.UUID
	hasIndex bool
}

// toNextKey returns a URL-safe pagination key for SearchEvents and
// SearchSessionEvents, to begin reading after the given (time, index, id)
// tuple. The timestamp is assumed to be precise to the microsecond, as that's
// the Postgres timestamptz granularity.
func toNextKey(t time.Time, index int64, id uuid.UUID) string {
	var b [8 + 8 + 16]byte
	binary.LittleEndian.PutUint64(b[0:8], uint64(t.UnixMicro()))
	binary.LittleEndian.PutUint64(b[8:16], uint64(index))
	copy(b[16:16+16], id[:])
	return base64.URLEncoding.EncodeToString(b[:])
}

// fromStartKey parses a URL-safe pagination key as returned by [toNextKey].
// It also accepts the legacy v18 format that only encoded time and id.
func fromStartKey(key string) (paginationKey, error) {
	b, err := base64.URLEncoding.DecodeString(key)
	if err != nil {
		return paginationKey{}, trace.Wrap(err)
	}

	switch len(b) {
	case 8 + 16:
		t := time.UnixMicro(int64(binary.LittleEndian.Uint64(b[0:8]))).UTC()
		var id uuid.UUID
		copy(id[:], b[8:8+16])
		return paginationKey{time: t, id: id}, nil
	case 8 + 8 + 16:
		t := time.UnixMicro(int64(binary.LittleEndian.Uint64(b[0:8]))).UTC()
		index := int64(binary.LittleEndian.Uint64(b[8:16]))
		var id uuid.UUID
		copy(id[:], b[16:16+16])
		return paginationKey{time: t, index: index, id: id, hasIndex: true}, nil
	default:
		return paginationKey{}, trace.BadParameter("malformed pagination key")
	}
}
