/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package utils

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
)

// TestTimeProtoConv verified the expected behavior of our zero-preserving
// proto timestamp conversion helpers.
func TestTimeProtoConv(t *testing.T) {
	t.Parallel()

	// verify the stated purpose of the conversion functions, preservation
	// of zeroness across the conversion boundary.
	require.Nil(t, TimeIntoProto(time.Time{}))
	require.True(t, TimeFromProto(nil).IsZero())
	require.Nil(t, TimeIntoProto(TimeFromProto(nil)))
	require.True(t, TimeFromProto(TimeIntoProto(time.Time{})).IsZero())

	// verify some relatively trivial examples. our conversion functions are
	// thin wrappers around the standard go/proto conversion functions, so
	// we aren't trying to robustly validate them, just add a sanity-check
	// against mistakes.
	tts := []time.Time{
		time.Unix(0, 0).UTC(),
		time.Unix(0, 1).UTC(),
		time.Unix(1, 0).UTC(),
		time.Unix(1, 1).UTC(),
		time.Now().UTC(),
	}

	for i, tt := range tts {
		protott := TimeIntoProto(tt)
		require.True(t, tt.Equal(TimeFromProto(protott)), "index %d", i)
		require.Empty(t, cmp.Diff(protott, TimeIntoProto(TimeFromProto(protott)), protocmp.Transform()))
	}
}
