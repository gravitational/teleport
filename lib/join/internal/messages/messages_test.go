// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package messages

import (
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

// TestClientParamsCheckForRole verifies that the payload matches the requested
// SystemRole. A wrong-typed or missing payload must be rejected with an error.
func TestClientParamsCheckForRole(t *testing.T) {
	t.Parallel()

	validExpires := time.Now().Add(time.Hour)
	validKeys := PublicKeys{
		PublicTLSKey: []byte("tls-key"),
		PublicSSHKey: []byte("ssh-key"),
	}
	validHostParams := func() *HostParams {
		return &HostParams{PublicKeys: validKeys, HostName: "node"}
	}
	validBotParams := func() *BotParams {
		return &BotParams{PublicKeys: validKeys, Expires: &validExpires}
	}

	tests := []struct {
		name   string
		role   types.SystemRole
		params ClientParams
		want   func(error) bool
	}{
		{
			name:   "instance with host params is valid",
			role:   types.RoleInstance,
			params: ClientParams{HostParams: validHostParams()},
			want:   nil,
		},
		{
			name:   "instance with bot params is rejected",
			role:   types.RoleInstance,
			params: ClientParams{BotParams: validBotParams()},
			want:   trace.IsBadParameter,
		},
		{
			name:   "instance with no payload is rejected",
			role:   types.RoleInstance,
			params: ClientParams{},
			want:   trace.IsBadParameter,
		},
		{
			name:   "bot with bot params is valid",
			role:   types.RoleBot,
			params: ClientParams{BotParams: validBotParams()},
			want:   nil,
		},
		{
			name:   "bot with host params is rejected",
			role:   types.RoleBot,
			params: ClientParams{HostParams: validHostParams()},
			want:   trace.IsBadParameter,
		},
		{
			name:   "bot with no payload is rejected",
			role:   types.RoleBot,
			params: ClientParams{},
			want:   trace.IsBadParameter,
		},
		{
			// Param contents are method-dependent (bound keypair omits HostName),
			// so CheckForRole must not reject a host payload for missing content.
			name:   "instance with minimal host params is accepted",
			role:   types.RoleInstance,
			params: ClientParams{HostParams: &HostParams{}},
			want:   nil,
		},
		{
			name:   "unsupported role is not implemented",
			role:   types.RoleProxy,
			params: ClientParams{HostParams: validHostParams()},
			want:   trace.IsNotImplemented,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.params.CheckForRole(tc.role)
			if tc.want == nil {
				require.NoError(t, err)
				return
			}
			require.True(t, tc.want(err), "unexpected error: %v", err)
		})
	}
}
