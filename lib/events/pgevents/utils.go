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

// toNextKey returns a URL-safe pagination key for SearchEvents and
// SearchSessionEvents, to begin reading after the given (time, id) pair. The
// timestamp is assumed to be precise to the microsecond, as that's the Postgres
// timestamptz granularity.
func toNextKey(t time.Time, id uuid.UUID) string {
	var b [8 + 16]byte
	binary.LittleEndian.PutUint64(b[0:8], uint64(t.UnixMicro()))
	copy(b[8:8+16], id[:])
	return base64.URLEncoding.EncodeToString(b[:])
}

// fromStartKey parses a URL-safe pagination key as returned by [toNextKey] into
// a (time, id) pair.
func fromStartKey(key string) (time.Time, uuid.UUID, error) {
	b, err := base64.URLEncoding.DecodeString(key)
	if err != nil {
		return time.Time{}, uuid.Nil, trace.Wrap(err)
	}
	if len(b) != 8+16 {
		return time.Time{}, uuid.Nil, trace.BadParameter("malformed pagination key")
	}

	t := time.UnixMicro(int64(binary.LittleEndian.Uint64(b[0:8]))).UTC()
	var id uuid.UUID
	copy(id[:], b[8:8+16])

	return t, id, nil
}
