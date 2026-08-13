/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package common

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
)

// TestReusableMFA_CaptureAfterFallback verifies that
// a MULTI ceremony result arriving after the fallback to the legacy requester is not stored.
func TestReusableMFA_CaptureAfterFallback(t *testing.T) {
	t.Parallel()

	m := newReusableMFA()
	m.FallbackToLegacy(t.Context(), trace.AccessDenied("reuse is not permitted"))

	m.Capture(&proto.MFAAuthenticateResponse{})
	requester, response := m.State()
	require.Equal(t, proto.UserCertsRequest_TSH_KUBE_LOCAL_PROXY, requester)
	require.Nil(t, response)
}

// TestReusableMFA_ClearKeepsFresherResponse verifies that
// clearing a stale response does not drop the replacement a peer already captured.
func TestReusableMFA_ClearKeepsFresherResponse(t *testing.T) {
	t.Parallel()

	m := newReusableMFA()
	stale := &proto.MFAAuthenticateResponse{}
	fresh := &proto.MFAAuthenticateResponse{}
	m.Capture(stale)
	m.Capture(fresh)

	m.Clear(stale)
	_, response := m.State()
	require.Same(t, fresh, response)

	m.Clear(fresh)
	_, response = m.State()
	require.Nil(t, response)
}
