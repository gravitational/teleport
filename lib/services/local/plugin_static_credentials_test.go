/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
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
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	creds, err = service.GetPluginStaticCredentialsByLabels(ctx, map[string]string{
		"label2": "value2",
	})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.PluginStaticCredentials{cred1, cred2}, creds,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	creds, err = service.GetPluginStaticCredentialsByLabels(ctx, map[string]string{
		"label2": "value2",
		"label3": "value3",
	})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.PluginStaticCredentials{cred2}, creds,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
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
