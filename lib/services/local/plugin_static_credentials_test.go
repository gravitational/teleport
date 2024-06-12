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

package local

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
)

// TestPluginStaticCredentialsCRUD tests backend operations with plugin static credentials resources.
func TestPluginStaticCredentialsCRUD(t *testing.T) {
	ctx := context.Background()

	backend, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	service, err := NewPluginStaticCredentialsService(backend)
	require.NoError(t, err)

	// Create a couple plugin static credentials.
	cred1, err := types.NewPluginStaticCredentials(
		types.Metadata{
			Name: "cred1",
			Labels: map[string]string{
				"label1": "value1",
				"label2": "value2",
			},
		},
		types.PluginStaticCredentialsSpecV1{
			Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
				APIToken: "some-token",
			},
		})
	require.NoError(t, err)
	cred2, err := types.NewPluginStaticCredentials(
		types.Metadata{
			Name: "cred2",
			Labels: map[string]string{
				"label2": "value2",
				"label3": "value3",
			},
		},
		types.PluginStaticCredentialsSpecV1{
			Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
				APIToken: "some-token",
			},
		})
	require.NoError(t, err)

	// Create both plugin static credentials.
	err = service.CreatePluginStaticCredentials(ctx, cred1)
	require.NoError(t, err)
	err = service.CreatePluginStaticCredentials(ctx, cred2)
	require.NoError(t, err)

	// Fetch static credentials by name.
	cred, err := service.GetPluginStaticCredentials(ctx, cred1.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(cred1, cred,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
	))

	// Try to fetch a static credential that doesn't exist.
	_, err = service.GetPluginStaticCredentials(ctx, "doesnotexist")
	require.True(t, trace.IsNotFound(err))

	// Try to create the same static credential.
	err = service.CreatePluginStaticCredentials(ctx, cred1)
	require.True(t, trace.IsAlreadyExists(err))

	// Fetch static credentials by label.
	creds, err := service.GetPluginStaticCredentialsByLabels(ctx, map[string]string{
		"label1": "value1",
		"label2": "value2",
	})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.PluginStaticCredentials{cred1}, creds,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
	))

	creds, err = service.GetPluginStaticCredentialsByLabels(ctx, map[string]string{
		"label2": "value2",
	})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.PluginStaticCredentials{cred1, cred2}, creds,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
	))

	creds, err = service.GetPluginStaticCredentialsByLabels(ctx, map[string]string{
		"label2": "value2",
		"label3": "value3",
	})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.PluginStaticCredentials{cred2}, creds,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
	))

	// Delete a static credential.
	err = service.DeletePluginStaticCredentials(ctx, cred1.GetName())
	require.NoError(t, err)
	_, err = service.GetPluginStaticCredentials(ctx, cred1.GetName())
	require.True(t, trace.IsNotFound(err))

	// Try to delete a static credential that doesn't exist.
	err = service.DeletePluginStaticCredentials(ctx, "doesnotexist")
	require.True(t, trace.IsNotFound(err))
}
