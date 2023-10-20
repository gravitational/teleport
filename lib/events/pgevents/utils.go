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
