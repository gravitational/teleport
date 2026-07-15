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

package eventstest

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/session"
)

func TestReplayObjectRoundTripAndRange(t *testing.T) {
	ctx := context.Background()
	u := NewMemoryUploader()
	sid := session.ID("beam-1")
	_, err := u.UploadReplayObject(ctx, sid, "blob.0", bytes.NewReader([]byte("0123456789")))
	require.NoError(t, err)

	rc, err := u.StreamReplayObjectRange(ctx, sid, "blob.0", 0, 0)
	require.NoError(t, err)
	got, _ := io.ReadAll(rc)
	rc.Close()
	require.Equal(t, "0123456789", string(got))

	rc, err = u.StreamReplayObjectRange(ctx, sid, "blob.0", 3, 4)
	require.NoError(t, err)
	got, _ = io.ReadAll(rc)
	rc.Close()
	require.Equal(t, "3456", string(got))

	_, err = u.StreamReplayObjectRange(ctx, sid, "missing", 0, 0)
	require.True(t, trace.IsNotFound(err))
}
