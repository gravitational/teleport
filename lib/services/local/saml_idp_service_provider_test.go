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

// TestSAMLIdPServiceProviderCRUD tests backend operations with SAML IdP service provider resources.
func TestSAMLIdPServiceProviderCRUD(t *testing.T) {
	ctx := context.Background()

	backend, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	service := NewSAMLIdPServiceProviderService(backend)

	// Create a couple service providers.
	sp1, err := types.NewSAMLIdPServiceProvider("sp1",
		types.SAMLIdPServiceProviderSpecV1{
			EntityDescriptor: "<valid />",
		})
	require.NoError(t, err)
	sp2, err := types.NewSAMLIdPServiceProvider("sp2",
		types.SAMLIdPServiceProviderSpecV1{
			EntityDescriptor: "<valid />",
		})
	require.NoError(t, err)

	// Initially we expect no service providers.
	out, err := service.GetSAMLIdPServiceProviders(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, len(out))

	// Create both service providers.
	err = service.CreateSAMLIdPServiceProvider(ctx, sp1)
	require.NoError(t, err)
	err = service.CreateSAMLIdPServiceProvider(ctx, sp2)
	require.NoError(t, err)

	// Fetch all service providers.
	out, err = service.GetSAMLIdPServiceProviders(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.SAMLIdPServiceProvider{sp1, sp2}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Fetch a specific service provider.
	sp, err := service.GetSAMLIdPServiceProvider(ctx, sp2.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(sp2, sp,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Try to fetch a service provider that doesn't exist.
	_, err = service.GetSAMLIdPServiceProvider(ctx, "doesnotexist")
	require.IsType(t, trace.NotFound(""), err)

	// Try to create the same service provider.
	err = service.CreateSAMLIdPServiceProvider(ctx, sp1)
	require.IsType(t, trace.AlreadyExists(""), err)

	// Update a service provider.
	sp1.SetEntityDescriptor("<updated />")
	err = service.UpdateSAMLIdPServiceProvider(ctx, sp1)
	require.NoError(t, err)
	sp, err = service.GetSAMLIdPServiceProvider(ctx, sp1.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(sp1, sp,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Delete a service provider.
	err = service.DeleteSAMLIdPServiceProvider(ctx, sp1.GetName())
	require.NoError(t, err)
	out, err = service.GetSAMLIdPServiceProviders(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.SAMLIdPServiceProvider{sp2}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Try to delete a service provider that doesn't exist.
	err = service.DeleteSAMLIdPServiceProvider(ctx, "doesnotexist")
	require.IsType(t, trace.NotFound(""), err)

	// Delete all service providers.
	err = service.DeleteAllSAMLIdPServiceProviders(ctx)
	require.NoError(t, err)
	out, err = service.GetSAMLIdPServiceProviders(ctx)
	require.NoError(t, err)
	require.Len(t, out, 0)
}
