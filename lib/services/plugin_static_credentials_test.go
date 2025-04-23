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

package services

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestMarshalPluginStaticCredentialsRoundTrip(t *testing.T) {
	spec := types.PluginStaticCredentialsSpecV1{
		Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
			APIToken: "some-token",
		},
	}

	creds, err := types.NewPluginStaticCredentials(types.Metadata{
		Name: "test-creds",
	}, spec)
	require.NoError(t, err)

	payload, err := MarshalPluginStaticCredentials(creds)
	require.NoError(t, err)

	unmarshaled, err := UnmarshalPluginStaticCredentials(payload)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(creds, unmarshaled))

	// TODO(greedy52) PluginStaticCredentials uses protojson for marshaling
	// and unmarshaling. Unfortunately Expires (*time.Time) is not supported
	// and becomes zero after the marshaling. Unlikely Expires will ever be
	// needed for PluginStaticCredentials in production but this can cause
	// unexpected issues in other places (like resource.SetExpiry in cache
	// tests).
	t.Run("with expires", func(t *testing.T) {
		t.Skip("PluginStaticCredentials does not marshal Expires")
		now := time.Now()

		creds.SetExpiry(now)
		payload, err := MarshalPluginStaticCredentials(creds)
		require.NoError(t, err)

		unmarshaled, err := UnmarshalPluginStaticCredentials(payload)
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(creds, unmarshaled))
		require.Equal(t, now, unmarshaled.Expiry())
	})
}
