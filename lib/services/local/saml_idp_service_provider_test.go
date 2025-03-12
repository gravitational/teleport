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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/crewjam/saml"
	"github.com/crewjam/saml/samlsp"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/services"
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
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
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
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	// Fetch a specific service provider.
	sp, err := service.GetSAMLIdPServiceProvider(ctx, sp2.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(sp2, sp,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
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
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
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
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
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

func TestCreateSAMLIdPServiceProvider_fetchOrGenerateEntityDescriptor(t *testing.T) {
	ctx := context.Background()
	backend, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	// First, test that given an empty entity descriptor and metadata serving Service Provider,
	// we can fetch and set entity descriptor using given entity ID.
	var spServerRespondedMetadata string

	testSPServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.RequestURI {
		case "/status-not-ok":
			w.WriteHeader(http.StatusNotFound)
		default:
			fullURL := fmt.Sprintf("https://%s", r.Host)
			// the metadata format returned by newEntityDescriptor is different from
			// what is generated by using saml.ServiceProvider.Metadata(). So the difference
			// is another useful indication that tells the entity descriptor was fetched rather
			// than generated.
			spServerRespondedMetadata = newEntityDescriptor(fullURL)
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, spServerRespondedMetadata)
		}

	}))
	defer testSPServer.Close()

	// new service provider with empty entity descriptor
	sp, err := types.NewSAMLIdPServiceProvider(
		types.Metadata{
			Name: "sp",
		},
		types.SAMLIdPServiceProviderSpecV1{
			EntityDescriptor: "",
			EntityID:         testSPServer.URL,
			ACSURL:           testSPServer.URL,
		})
	require.NoError(t, err)

	service, err := NewSAMLIdPServiceProviderService(backend, WithHTTPClient(testSPServer.Client()))
	require.NoError(t, err)

	err = service.CreateSAMLIdPServiceProvider(ctx, sp)
	require.NoError(t, err)

	spFromBackend, err := service.GetSAMLIdPServiceProvider(ctx, sp.GetName())
	require.NoError(t, err)

	require.Equal(t, strings.TrimSpace(spServerRespondedMetadata), strings.TrimSpace(spFromBackend.GetEntityDescriptor()))

	err = service.DeleteSAMLIdPServiceProvider(ctx, sp.GetName())
	require.NoError(t, err)

	// Now test that given an empty entity descriptor and Service Provider which does not
	// respond to metadata requests, generateAndSetEntityDescriptor() is called and default entity descriptor is set
	// with provided entity ID and ACS URL.

	// new service provider with empty entity descriptor
	notFoundURL := fmt.Sprintf("%s/status-not-ok", testSPServer.URL)
	sp2, err := types.NewSAMLIdPServiceProvider(
		types.Metadata{
			Name: "sp",
		},
		types.SAMLIdPServiceProviderSpecV1{
			EntityDescriptor: "",
			EntityID:         notFoundURL,
			ACSURL:           testSPServer.URL,
		})
	require.NoError(t, err)

	service2, err := NewSAMLIdPServiceProviderService(backend)
	require.NoError(t, err)

	err = service2.CreateSAMLIdPServiceProvider(ctx, sp2)
	require.NoError(t, err)

	sp2FromBackend, err := service2.GetSAMLIdPServiceProvider(ctx, sp2.GetName())
	require.NoError(t, err)

	metadataTemplate := services.NewSAMLTestSPMetadata(notFoundURL, testSPServer.URL)

	expected, err := samlsp.ParseMetadata([]byte(metadataTemplate))
	require.NoError(t, err)

	actual, err := samlsp.ParseMetadata([]byte(sp2FromBackend.GetEntityDescriptor()))
	require.NoError(t, err)

	// ignoring ValidUntil as its value (duration) is set using time.Now() and comparing it will create flaky test.
	require.Empty(t, cmp.Diff(expected, actual,
		cmpopts.IgnoreFields(saml.EntityDescriptor{}, "ValidUntil"),
		cmpopts.IgnoreFields(saml.SPSSODescriptor{}, "ValidUntil"),
	))
}

func TestCreateSAMLIdPServiceProvider_fetchAndSetEntityDescriptor(t *testing.T) {
	testSPServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.RequestURI {
		case "/status-not-ok":
			w.WriteHeader(http.StatusNotFound)
		case "/status-302-found":
			w.WriteHeader(http.StatusFound)
		case "/invalid-metadata":
			fmt.Fprintln(w, "test")
		default:
			location := fmt.Sprintf("https://%s", r.Host)
			metadata := services.NewSAMLTestSPMetadata(location, location)
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, metadata)
		}
	}))
	defer testSPServer.Close()

	cases := []struct {
		name     string
		entityID string
		wantErr  bool
	}{
		{
			name:     "status not ok",
			entityID: fmt.Sprintf("%s/status-not-ok", testSPServer.URL),
			wantErr:  true,
		},
		{
			name:     "status 302 found",
			entityID: fmt.Sprintf("%s/status-302-found", testSPServer.URL),
			wantErr:  true,
		},
		{
			name:     "invalid metadata",
			entityID: fmt.Sprintf("%s/invalid-metadata", testSPServer.URL),
			wantErr:  true,
		},
		{
			name:     "correct response code and metadata response",
			entityID: fmt.Sprintf("%s/saml/metadata", testSPServer.URL),
			wantErr:  false,
		},
	}

	ctx := context.Background()

	backend, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			idpSPService, err := NewSAMLIdPServiceProviderService(backend, WithHTTPClient(testSPServer.Client()))
			require.NoError(t, err)

			sp, err := types.NewSAMLIdPServiceProvider(
				types.Metadata{
					Name: test.entityID,
				},
				types.SAMLIdPServiceProviderSpecV1{
					EntityDescriptor: "",
					EntityID:         test.entityID,
					ACSURL:           test.entityID,
				})
			require.NoError(t, err)

			err = idpSPService.fetchAndSetEntityDescriptor(sp)
			if test.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotEmpty(t, sp.GetEntityDescriptor())
			}
		})
	}
}

func TestCreateSAMLIdPServiceProvider_embedAttributeMapping(t *testing.T) {
	ctx := context.Background()

	backend, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	idpSPService, err := NewSAMLIdPServiceProviderService(backend)
	require.NoError(t, err)

	// 1. Verify attribute embedding works.
	attributeMappingInput := []*types.SAMLAttributeMapping{
		{
			Name:       "username",
			Value:      "user.spec.metadata.username",
			NameFormat: "urn:oasis:names:tc:SAML:2.0:attrname-format:unspecified",
		},
		{
			Name:       "groups",
			NameFormat: "urn:oasis:names:tc:SAML:2.0:attrname-format:unspecified",
			Value:      "user.spec.traits.groups",
		},
	}
	sp, err := types.NewSAMLIdPServiceProvider(
		types.Metadata{
			Name: "sp",
		},
		types.SAMLIdPServiceProviderSpecV1{
			EntityID:         "sp",
			ACSURL:           "https://example.com",
			AttributeMapping: attributeMappingInput,
		})
	require.NoError(t, err)

	err = idpSPService.CreateSAMLIdPServiceProvider(ctx, sp)
	require.NoError(t, err)

	spFromBackend, err := idpSPService.GetSAMLIdPServiceProvider(ctx, sp.GetName())
	require.NoError(t, err)

	edWithEmbeddedAttributes, err := samlsp.ParseMetadata([]byte(spFromBackend.GetEntityDescriptor()))
	require.NoError(t, err)

	_, teleportSPSSODescriptor := GetTeleportSPSSODescriptor(edWithEmbeddedAttributes.SPSSODescriptors)
	embeddedAttributes := teleportSPSSODescriptorToAttributeMapping(teleportSPSSODescriptor)
	require.Equal(t, attributeMappingInput, embeddedAttributes)
	require.Contains(t, spFromBackend.GetEntityDescriptor(), `<ServiceName xml:lang="">teleport_saml_idp_service</ServiceName>`)

	// 2. Now using previously embedded entity descriptor (spFromBackend.GetEntityDescriptor())
	// update sp with one additional attribute mapping and test that:
	// - the new attribute mapping is added to SPSSOdescriptor.
	// - there is exactly one copy of embedded SPSSOdescriptor.
	sp.SetEntityDescriptor(spFromBackend.GetEntityDescriptor())
	attributeMappingInput = append(attributeMappingInput, &types.SAMLAttributeMapping{
		Name:       "firstname",
		Value:      "user.spec.traits.firstname",
		NameFormat: "urn:oasis:names:tc:SAML:2.0:attrname-format:unspecified",
	})
	sp.SetAttributeMapping(attributeMappingInput)

	err = idpSPService.UpdateSAMLIdPServiceProvider(ctx, sp)
	require.NoError(t, err)
	updatedSPFromBackend, err := idpSPService.GetSAMLIdPServiceProvider(ctx, sp.GetName())
	require.NoError(t, err)
	edWithUpdatedAttributes, err := samlsp.ParseMetadata([]byte(updatedSPFromBackend.GetEntityDescriptor()))
	require.NoError(t, err)
	_, teleportSPSSODescriptor = GetTeleportSPSSODescriptor(edWithUpdatedAttributes.SPSSODescriptors)
	embeddedAttributes = teleportSPSSODescriptorToAttributeMapping(teleportSPSSODescriptor)
	require.Equal(t, attributeMappingInput, embeddedAttributes)
	require.Equal(t, 1, countEmbeddedSPSSODescriptor(edWithUpdatedAttributes.SPSSODescriptors))

	// 3. test we do not override SPSSODescriptor that isn't embedded by Teleport.
	// We will create SP with test entity descriptor edWithMultipleSPSSODescriptor, which already
	// has total 2 SPSSODescriptor elements. Once the SP is created with attribute mapping,
	// we will test that attributes are correctly embedded and that total count of embedded
	// SPSSODescriptor is exactly 1 and if we remove embedded element, the resulting entity descriptor
	// matches original xml.
	newMultipleSSODescriptorSP, err := types.NewSAMLIdPServiceProvider(
		types.Metadata{
			Name: "newMultipleSSODescriptorSP",
		},
		types.SAMLIdPServiceProviderSpecV1{
			EntityDescriptor: edWithMultipleSPSSODescriptor,
			AttributeMapping: attributeMappingInput,
		})
	require.NoError(t, err)

	err = idpSPService.CreateSAMLIdPServiceProvider(ctx, newMultipleSSODescriptorSP)
	require.NoError(t, err)
	newMultipleSSODescriptorSPFromBackend, err := idpSPService.GetSAMLIdPServiceProvider(ctx, newMultipleSSODescriptorSP.GetName())
	require.NoError(t, err)
	edWithEmbeddedAttributes, err = samlsp.ParseMetadata([]byte(newMultipleSSODescriptorSPFromBackend.GetEntityDescriptor()))
	require.NoError(t, err)
	_, teleportSPSSODescriptor = GetTeleportSPSSODescriptor(edWithEmbeddedAttributes.SPSSODescriptors)
	embeddedAttributes = teleportSPSSODescriptorToAttributeMapping(teleportSPSSODescriptor)
	require.Equal(t, attributeMappingInput, embeddedAttributes)
	require.Equal(t, 1, countEmbeddedSPSSODescriptor(edWithEmbeddedAttributes.SPSSODescriptors))

	// and if we remove attribute mapping, the resulting entity descriptor should be exactly the same as before embedding.
	newMultipleSSODescriptorSP.SetAttributeMapping(nil)
	err = idpSPService.UpdateSAMLIdPServiceProvider(ctx, newMultipleSSODescriptorSP)
	require.NoError(t, err)
	newMultipleSSODescriptorSPFromBackend, err = idpSPService.GetSAMLIdPServiceProvider(ctx, newMultipleSSODescriptorSP.GetName())
	require.NoError(t, err)
	edWithoutEmbeddedAttributes, err := samlsp.ParseMetadata([]byte(newMultipleSSODescriptorSPFromBackend.GetEntityDescriptor()))
	require.NoError(t, err)
	originalED, err := samlsp.ParseMetadata([]byte(edWithMultipleSPSSODescriptor))
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(originalED, edWithoutEmbeddedAttributes))
}

func teleportSPSSODescriptorToAttributeMapping(spSSODescriptors saml.SPSSODescriptor) (embeddedAttributes []*types.SAMLAttributeMapping) {
	for _, acs := range spSSODescriptors.AttributeConsumingServices {
		for _, reqAttr := range acs.RequestedAttributes {
			embeddedAttributes = append(embeddedAttributes, &types.SAMLAttributeMapping{
				Name:       reqAttr.Name,
				NameFormat: reqAttr.NameFormat,
				Value:      reqAttr.Values[0].Value,
			})
		}
	}
	return
}

func countEmbeddedSPSSODescriptor(spSSODescriptor []saml.SPSSODescriptor) (embeddedSPSSODescriptorAcount int) {
	for _, ssoDescriptor := range spSSODescriptor {
		for _, acs := range ssoDescriptor.AttributeConsumingServices {
			for _, serviceName := range acs.ServiceNames {
				if serviceName.Value == samlIDPServiceName {
					embeddedSPSSODescriptorAcount++
				}
			}
		}
	}
	return
}

const edWithMultipleSPSSODescriptor = `<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" validUntil="2023-12-10T16:26:51.083Z" entityID="newMultipleSSODescriptorSP">
<SPSSODescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" validUntil="2023-12-10T16:26:51.083029Z" protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol" AuthnRequestsSigned="false" WantAssertionsSigned="true">
	<NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified</NameIDFormat>
	<AssertionConsumerService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="https://example.com" index="1"></AssertionConsumerService>
	<AssertionConsumerService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Artifact" Location="https://example.com" index="2"></AssertionConsumerService>
</SPSSODescriptor>
<SPSSODescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" protocolSupportEnumeration="">
	<AttributeConsumingService index="0">
		<RequestedAttribute FriendlyName="displayname" Name="displayname" NameFormat="urn:oasis:names:tc:SAML:2.0:attrname-format:unspecified">
			<AttributeValue xmlns:_XMLSchema-instance="http://www.w3.org/2001/XMLSchema-instance" _XMLSchema-instance:type="">user.spec.traits.username</AttributeValue>
		</RequestedAttribute>
		<RequestedAttribute FriendlyName="lastName" Name="lastName" NameFormat="urn:oasis:names:tc:SAML:2.0:attrname-format:basic">
			<AttributeValue xmlns:_XMLSchema-instance="http://www.w3.org/2001/XMLSchema-instance" _XMLSchema-instance:type="">user.spec.traits.lastname</AttributeValue>
		</RequestedAttribute>
	</AttributeConsumingService>
</SPSSODescriptor>
<SPSSODescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" protocolSupportEnumeration="">
	<AttributeConsumingService index="0">
		<RequestedAttribute FriendlyName="displayname" Name="displayname" NameFormat="urn:oasis:names:tc:SAML:2.0:attrname-format:unspecified">
			<AttributeValue xmlns:_XMLSchema-instance="http://www.w3.org/2001/XMLSchema-instance" _XMLSchema-instance:type="">user.spec.traits.username</AttributeValue>
		</RequestedAttribute>
		<RequestedAttribute FriendlyName="lastName" Name="lastName" NameFormat="urn:oasis:names:tc:SAML:2.0:attrname-format:basic">
			<AttributeValue xmlns:_XMLSchema-instance="http://www.w3.org/2001/XMLSchema-instance" _XMLSchema-instance:type="">user.spec.traits.lastname</AttributeValue>
		</RequestedAttribute>
	</AttributeConsumingService>
</SPSSODescriptor>
</EntityDescriptor>
`

func TestCreateSAMLIdPServiceProvider_GetTeleportSPSSODescriptor(t *testing.T) {
	// edWithMultipleSPSSODescriptor has predefined two ACS elements
	ed, err := samlsp.ParseMetadata([]byte(edWithMultipleSPSSODescriptor))
	require.NoError(t, err)

	ed.SPSSODescriptors = append(ed.SPSSODescriptors, saml.SPSSODescriptor{
		AttributeConsumingServices: []saml.AttributeConsumingService{
			{
				// ACS with the teleport_saml_idp_service name
				ServiceNames: []saml.LocalizedName{{Value: samlIDPServiceName}},
				RequestedAttributes: []saml.RequestedAttribute{{
					Attribute: saml.Attribute{
						FriendlyName: "groups",
						Name:         "groups",
						NameFormat:   "urn:oasis:names:tc:SAML:2.0:attrname-format:unspecified",
						Values:       []saml.AttributeValue{{Value: "user.spec.traits.groups"}},
					},
				}},
			},
			{
				// ACS without the teleport_saml_idp_service service name
				RequestedAttributes: []saml.RequestedAttribute{{
					Attribute: saml.Attribute{
						FriendlyName: "roles",
						Name:         "roles",
						NameFormat:   "urn:oasis:names:tc:SAML:2.0:attrname-format:unspecified",
						Values:       []saml.AttributeValue{{Value: "user.spec.traits.grrolesoups"}},
					},
				}},
			},
		},
	})
	index, _ := GetTeleportSPSSODescriptor(ed.SPSSODescriptors)
	require.Equal(t, 3, index)
}

func TestDeleteSAMLServiceProviderWhenReferencedByPlugin(t *testing.T) {
	ctx := context.Background()
	backend, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clockwork.NewFakeClock(),
	})
	require.NoError(t, err)
	samlService, err := NewSAMLIdPServiceProviderService(backend)
	require.NoError(t, err)
	pluginService := NewPluginsService(backend)

	sp, err := types.NewSAMLIdPServiceProvider(
		types.Metadata{
			Name: "sp",
		},
		types.SAMLIdPServiceProviderSpecV1{
			EntityDescriptor: newEntityDescriptor("sp"),
			EntityID:         "sp",
		})
	require.NoError(t, err)
	require.NoError(t, samlService.CreateSAMLIdPServiceProvider(ctx, sp))

	// service provider should not be deleted when referenced by the plugin.
	require.NoError(t, pluginService.CreatePlugin(ctx, fixtures.NewIdentityCenterPlugin(t, sp.GetName(), sp.GetName())))
	err = samlService.DeleteSAMLIdPServiceProvider(ctx, sp.GetName())
	require.ErrorContains(t, err, "referenced by AWS Identity Center integration")

	// service provider should be deleted once the referenced plugin itself is deleted.
	// other existing plugin should not prevent SAML service provider from deletion.
	require.NoError(t, pluginService.CreatePlugin(ctx, fixtures.NewMattermostPlugin(t)))
	require.NoError(t, pluginService.DeletePlugin(ctx, types.PluginTypeAWSIdentityCenter))
	require.NoError(t, samlService.DeleteSAMLIdPServiceProvider(ctx, sp.GetName()))
}
