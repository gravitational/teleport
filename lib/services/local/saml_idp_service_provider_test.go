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
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
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

	// Try to create a service provider with an invalid acs.
	invalidSP, err := types.NewSAMLIdPServiceProvider(
		types.Metadata{
			Name: "sp3",
		},
		types.SAMLIdPServiceProviderSpecV1{
			EntityDescriptor: newInvalidACSEntityDescriptor("sp1"),
			EntityID:         "sp1",
		})
	require.NoError(t, err)
	err = service.CreateSAMLIdPServiceProvider(ctx, invalidSP)
	assert.Error(t, err)
	require.True(t, trace.IsBadParameter(err), "CreateSAMLIdPServiceProvider error mismatch, wanted BadParameter, got %q (%T)", err, trace.Unwrap(err))

	// Try to create a service provider with a http acs.
	invalidSP, err = types.NewSAMLIdPServiceProvider(
		types.Metadata{
			Name: "sp3",
		},
		types.SAMLIdPServiceProviderSpecV1{
			EntityDescriptor: newHTTPACSEntityDescriptor("sp1"),
			EntityID:         "sp1",
		})
	require.NoError(t, err)
	err = service.CreateSAMLIdPServiceProvider(ctx, invalidSP)
	assert.Error(t, err)
	require.True(t, trace.IsBadParameter(err), "CreateSAMLIdPServiceProvider error mismatch, wanted BadParameter, got %q (%T)", err, trace.Unwrap(err))

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
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
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
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
	))

	// Fetch a specific service provider.
	sp, err := service.GetSAMLIdPServiceProvider(ctx, sp2.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(sp2, sp,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
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
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
	))

	// Update a service provider to an existing entity ID.
	sp, err = service.GetSAMLIdPServiceProvider(ctx, sp1.GetName())
	require.NoError(t, err)
	sp.SetEntityDescriptor(newEntityDescriptor("sp2"))
	sp.SetEntityID("sp2")
	err = service.UpdateSAMLIdPServiceProvider(ctx, sp)
	require.Error(t, err)

	// Update a service provider with an invalid acs.
	invalidSP, err = service.GetSAMLIdPServiceProvider(ctx, sp1.GetName())
	require.NoError(t, err)
	invalidSP.SetEntityDescriptor(newInvalidACSEntityDescriptor(invalidSP.GetEntityID()))
	err = service.UpdateSAMLIdPServiceProvider(ctx, invalidSP)
	assert.Error(t, err)
	require.True(t, trace.IsBadParameter(err), "CreateSAMLIdPServiceProvider error mismatch, wanted BadParameter, got %q (%T)", err, trace.Unwrap(err))

	// Update a service provider with a http acs.
	invalidSP.SetEntityDescriptor(newHTTPACSEntityDescriptor(invalidSP.GetEntityID()))
	err = service.UpdateSAMLIdPServiceProvider(ctx, invalidSP)
	assert.Error(t, err)
	require.True(t, trace.IsBadParameter(err), "UpdateSAMLIdPServiceProvider error mismatch, wanted BadParameter, got %q (%T)", err, trace.Unwrap(err))

	// Delete a service provider.
	err = service.DeleteSAMLIdPServiceProvider(ctx, sp1.GetName())
	require.NoError(t, err)
	out, nextToken, err = service.ListSAMLIdPServiceProviders(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Empty(t, cmp.Diff([]types.SAMLIdPServiceProvider{sp2}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
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

func TestValidateSAMLIdPServiceProvider(t *testing.T) {
	descriptor := newEntityDescriptor("IAMShowcase")

	cases := []struct {
		name         string
		spec         types.SAMLIdPServiceProviderSpecV1
		errAssertion require.ErrorAssertionFunc
	}{
		{
			name: "valid provider",
			spec: types.SAMLIdPServiceProviderSpecV1{
				EntityDescriptor: descriptor,
				EntityID:         "IAMShowcase",
			},
			errAssertion: require.NoError,
		},
		{
			name: "invalid entity id",
			spec: types.SAMLIdPServiceProviderSpecV1{
				EntityDescriptor: descriptor,
				EntityID:         uuid.NewString(),
			},
			errAssertion: require.Error,
		},
		{
			name: "invalid acs",
			spec: types.SAMLIdPServiceProviderSpecV1{
				EntityDescriptor: newInvalidACSEntityDescriptor("IAMShowcase"),
				EntityID:         "IAMShowcase",
			},
			errAssertion: require.Error,
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			sp, err := types.NewSAMLIdPServiceProvider(types.Metadata{Name: "sp"}, test.spec)
			require.NoError(t, err)
			test.errAssertion(t, validateSAMLIdPServiceProvider(sp))
		})
	}
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

func newInvalidACSEntityDescriptor(entityID string) string {
	return fmt.Sprintf(invalidEntityDescriptor, entityID)
}

// A test entity descriptor from https://sptest.iamshowcase.com/testsp_metadata.xml with invalid ACS locations.
const invalidEntityDescriptor = `<?xml version="1.0" encoding="UTF-8"?>
<md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" xmlns:ds="http://www.w3.org/2000/09/xmldsig#" entityID="%s" validUntil="2025-12-09T09:13:31.006Z">
   <md:SPSSODescriptor AuthnRequestsSigned="false" WantAssertionsSigned="true" protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
      <md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified</md:NameIDFormat>
      <md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress</md:NameIDFormat>
      <md:AssertionConsumerService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="javascript://sptest.iamshowcase.com/acs" index="0" isDefault="true"/>
   </md:SPSSODescriptor>
</md:EntityDescriptor>
`

func newHTTPACSEntityDescriptor(entityID string) string {
	return fmt.Sprintf(httpEntityDescriptor, entityID)
}

// A test entity descriptor from https://sptest.iamshowcase.com/testsp_metadata.xml with a http ACS locations.
const httpEntityDescriptor = `<?xml version="1.0" encoding="UTF-8"?>
<md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" xmlns:ds="http://www.w3.org/2000/09/xmldsig#" entityID="%s" validUntil="2025-12-09T09:13:31.006Z">
   <md:SPSSODescriptor AuthnRequestsSigned="false" WantAssertionsSigned="true" protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
      <md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified</md:NameIDFormat>
      <md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress</md:NameIDFormat>
      <md:AssertionConsumerService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="http://sptest.iamshowcase.com/acs" index="0" isDefault="true"/>
   </md:SPSSODescriptor>
</md:EntityDescriptor>
`
