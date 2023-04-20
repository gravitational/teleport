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
	"fmt"
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

	service, err := NewSAMLIdPServiceProviderService(backend)
	require.NoError(t, err)

	// Create a couple service providers.
	sp1, err := types.NewSAMLIdPServiceProvider(
		types.Metadata{
			Name: "sp1",
		},
		types.SAMLIdPServiceProviderSpecV1{
			EntityDescriptor: newEntityDescriptor("sp1"),
			EntityID:         "sp1",
		})
	require.NoError(t, err)
	sp2, err := types.NewSAMLIdPServiceProvider(
		types.Metadata{
			Name: "sp2",
		},
		types.SAMLIdPServiceProviderSpecV1{
			EntityDescriptor: newEntityDescriptor("sp2"),
			EntityID:         "sp2",
		})
	require.NoError(t, err)

	// Initially we expect no service providers.
	out, nextToken, err := service.ListSAMLIdPServiceProviders(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Empty(t, out)

	// Create both service providers.
	err = service.CreateSAMLIdPServiceProvider(ctx, sp1)
	require.NoError(t, err)
	err = service.CreateSAMLIdPServiceProvider(ctx, sp2)
	require.NoError(t, err)

	// Create a service provider with a duplicate entity ID.
	_, err = types.NewSAMLIdPServiceProvider(
		types.Metadata{
			Name: "sp3",
		},
		types.SAMLIdPServiceProviderSpecV1{
			EntityDescriptor: newEntityDescriptor("sp1"),
			EntityID:         "sp1",
		})
	require.NoError(t, err)
	err = service.CreateSAMLIdPServiceProvider(ctx, sp2)
	require.Error(t, err)

	// Create a service provider with an entity ID that doesn't match the metadata.
	_, err = types.NewSAMLIdPServiceProvider(
		types.Metadata{
			Name: "sp3",
		},
		types.SAMLIdPServiceProviderSpecV1{
			EntityDescriptor: newEntityDescriptor("sp3"),
			EntityID:         "some-other-entity-id",
		})
	require.NoError(t, err)
	err = service.CreateSAMLIdPServiceProvider(ctx, sp2)
	require.Error(t, err)

	// Fetch all service providers.
	out, nextToken, err = service.ListSAMLIdPServiceProviders(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Empty(t, cmp.Diff([]types.SAMLIdPServiceProvider{sp1, sp2}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Fetch a paginated list of service providers
	paginatedOut := make([]types.SAMLIdPServiceProvider, 0, 2)
	numPages := 0
	for {
		numPages++
		out, nextToken, err = service.ListSAMLIdPServiceProviders(ctx, 1, nextToken)
		require.NoError(t, err)

		paginatedOut = append(paginatedOut, out...)
		if nextToken == "" {
			break
		}
	}

	require.Equal(t, 2, numPages)
	require.Empty(t, cmp.Diff([]types.SAMLIdPServiceProvider{sp1, sp2}, paginatedOut,
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
	require.True(t, trace.IsNotFound(err))

	// Try to create the same service provider.
	err = service.CreateSAMLIdPServiceProvider(ctx, sp1)
	require.True(t, trace.IsAlreadyExists(err))

	// Update a service provider.
	sp1.SetEntityDescriptor(newEntityDescriptor("updated-sp1"))
	sp1.SetEntityID("updated-sp1")
	err = service.UpdateSAMLIdPServiceProvider(ctx, sp1)
	require.NoError(t, err)
	sp, err = service.GetSAMLIdPServiceProvider(ctx, sp1.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(sp1, sp,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Update a service provider to an existing entity ID.
	sp, err = service.GetSAMLIdPServiceProvider(ctx, sp1.GetName())
	require.NoError(t, err)
	sp.SetEntityDescriptor(newEntityDescriptor("sp2"))
	sp.SetEntityID("sp2")
	err = service.UpdateSAMLIdPServiceProvider(ctx, sp)
	require.Error(t, err)

	// Delete a service provider.
	err = service.DeleteSAMLIdPServiceProvider(ctx, sp1.GetName())
	require.NoError(t, err)
	out, nextToken, err = service.ListSAMLIdPServiceProviders(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Empty(t, cmp.Diff([]types.SAMLIdPServiceProvider{sp2}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Try to delete a service provider that doesn't exist.
	err = service.DeleteSAMLIdPServiceProvider(ctx, "doesnotexist")
	require.True(t, trace.IsNotFound(err))

	// Delete all service providers.
	err = service.DeleteAllSAMLIdPServiceProviders(ctx)
	require.NoError(t, err)
	out, nextToken, err = service.ListSAMLIdPServiceProviders(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Empty(t, out)
}

func newEntityDescriptor(entityID string) string {
	return fmt.Sprintf(testEntityDescriptor, entityID)
}

// A test entity descriptor from https://sptest.iamshowcase.com/testsp_metadata.xml.
const testEntityDescriptor = `<?xml version="1.0" encoding="UTF-8"?>
<md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" xmlns:ds="http://www.w3.org/2000/09/xmldsig#" entityID="%s" validUntil="2025-12-09T09:13:31.006Z">
   <md:SPSSODescriptor AuthnRequestsSigned="false" WantAssertionsSigned="true" protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
      <md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified</md:NameIDFormat>
      <md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress</md:NameIDFormat>
      <md:AssertionConsumerService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="https://sptest.iamshowcase.com/acs" index="0" isDefault="true"/>
   </md:SPSSODescriptor>
</md:EntityDescriptor>
`
