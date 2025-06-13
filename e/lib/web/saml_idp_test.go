package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	samlidpui "github.com/gravitational/teleport/e/lib/web/ui/samlidp"
)

func TestGetSAMLIdpServiceProviderHandle(t *testing.T) {
	s := newWebSuite(t)
	webPack := s.newAuthWebPack(t, "foo")
	authClient := s.newAdminAuthClient(s.ctx, t)

	sp1, err := types.NewSAMLIdPServiceProvider(
		types.Metadata{
			Name: "sp1",
		},
		types.SAMLIdPServiceProviderSpecV1{
			ACSURL:           "https://sp1",
			EntityID:         "https://sp1",
			EntityDescriptor: newEntityDescriptor("https://sp1", "https://sp1"),
		},
	)
	require.NoError(t, err)
	err = authClient.CreateSAMLIdPServiceProvider(context.Background(), sp1)
	require.NoError(t, err)

	sp1FromBackend, err := authClient.GetSAMLIdPServiceProvider(context.Background(), sp1.GetName())
	require.NoError(t, err)

	endpoint := webPack.clt.Endpoint("enterprise", "samlidp", sp1.GetName())
	resp, err := webPack.clt.Get(s.ctx, endpoint, url.Values{})
	require.NoError(t, err)

	require.Equal(t, sp1FromBackend, apiSPToProtoSP(t, resp.Bytes()))
}

func apiSPToProtoSP(t *testing.T, resp []byte) types.SAMLIdPServiceProvider {
	var uiSP types.SAMLIdPServiceProviderV1
	err := json.Unmarshal(resp, &uiSP)
	require.NoError(t, err)

	new, err := types.NewSAMLIdPServiceProvider(uiSP.Metadata, uiSP.Spec)
	require.NoError(t, err)

	return new
}

func TestCreateSAMLIdpServiceProviderHandle(t *testing.T) {
	s := newWebSuite(t)
	webPack := s.newAuthWebPack(t, "foo")
	endpoint := webPack.clt.Endpoint("enterprise", "samlidp")

	var testCases = []struct {
		name         string
		req          samlidpui.CreateSAMLIdPServiceProviderRequest
		errAssertion require.ErrorAssertionFunc
	}{
		{
			name: "empty app name",
			req: samlidpui.CreateSAMLIdPServiceProviderRequest{
				Name:             "",
				EntityID:         "",
				ACSURL:           "",
				AttributeMapping: []*types.SAMLAttributeMapping{},
			},
			errAssertion: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "missing parameter Name")
			},
		},
		{
			name: "missing entity descriptor and entity ID",
			req: samlidpui.CreateSAMLIdPServiceProviderRequest{
				Name:             "newSAMLApp",
				EntityID:         "",
				ACSURL:           "https://example.com/saml/acs",
				AttributeMapping: []*types.SAMLAttributeMapping{},
			},
			errAssertion: func(t require.TestingT, err error, i ...any) {
				require.ErrorIs(t, err, types.ErrEmptyEntityDescriptorAndEntityID)
			},
		},
		{
			name: "missing entity descriptor and ACS URL",
			req: samlidpui.CreateSAMLIdPServiceProviderRequest{
				Name:             "newSAMLApp",
				EntityID:         "https://example.com/saml/metadata",
				ACSURL:           "",
				AttributeMapping: []*types.SAMLAttributeMapping{},
			},
			errAssertion: func(t require.TestingT, err error, i ...any) {
				require.ErrorIs(t, err, types.ErrEmptyEntityDescriptorAndACSURL)
			},
		},
		{
			name: "missing attribute name",
			req: samlidpui.CreateSAMLIdPServiceProviderRequest{
				Name:             "newSAMLApp",
				EntityDescriptor: "",
				EntityID:         "https://example.com/saml/metadata",
				ACSURL:           "https://example.com/saml/metadata",
				AttributeMapping: []*types.SAMLAttributeMapping{{Name: "", NameFormat: "", Value: "user.spec.roles"}},
			},
			errAssertion: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "attribute name is required")
			},
		},
		{
			name: "missing attribute value",
			req: samlidpui.CreateSAMLIdPServiceProviderRequest{
				Name:             "newSAMLApp",
				EntityDescriptor: "",
				EntityID:         "https://example.com/saml/metadata",
				ACSURL:           "https://example.com/saml/metadata",
				AttributeMapping: []*types.SAMLAttributeMapping{{Name: "roles", NameFormat: "", Value: ""}},
			},
			errAssertion: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "attribute value is required")
			},
		},
		{
			name: "duplicate attribute name and value",
			req: samlidpui.CreateSAMLIdPServiceProviderRequest{
				Name:             "newSAMLApp",
				EntityDescriptor: "",
				EntityID:         "https://example.com/saml/metadata",
				ACSURL:           "https://example.com/saml/metadata",
				AttributeMapping: []*types.SAMLAttributeMapping{{Name: "roles", NameFormat: "", Value: "user.spec.roles"}, {Name: "roles", NameFormat: "", Value: "user.spec.roles"}},
			},
			errAssertion: func(t require.TestingT, err error, i ...any) {
				require.ErrorIs(t, err, types.ErrDuplicateAttributeName)
			},
		},
		{
			name: "unsupported preset name",
			req: samlidpui.CreateSAMLIdPServiceProviderRequest{
				Name:             "newSAMLApp",
				EntityDescriptor: "",
				EntityID:         "https://example.com/saml/metadata",
				ACSURL:           "https://example.com/saml/metadata",
				Preset:           "unsupported-preset",
			},
			errAssertion: func(t require.TestingT, err error, i ...any) {
				require.ErrorIs(t, err, types.ErrUnsupportedPresetName)
			},
		},
		{
			name: "valid request with only EntityID and ACSURL",
			req: samlidpui.CreateSAMLIdPServiceProviderRequest{
				Name:             "sp1",
				EntityDescriptor: "",
				EntityID:         "https://sp1/saml/metadata",
				ACSURL:           "https://sp1/saml/metadata",
				Preset:           "",
			},
			errAssertion: require.NoError,
		},
		{
			name: "valid request with onlyEntityDescriptor",
			req: samlidpui.CreateSAMLIdPServiceProviderRequest{
				Name:             "sp2",
				EntityDescriptor: newEntityDescriptor("https://sp2", "https://sp2"),
				Preset:           "",
			},
			errAssertion: require.NoError,
		},
		{
			name: "valid request with attribute mapping",
			req: samlidpui.CreateSAMLIdPServiceProviderRequest{
				Name:             "sp3",
				EntityDescriptor: newEntityDescriptor("https://sp3", "https://sp3"),
				AttributeMapping: []*types.SAMLAttributeMapping{
					{
						Name:  "username",
						Value: "user.traits.name",
					},
					{
						Name:  "user1",
						Value: "user.traits.givenname",
					},
				},
			},
			errAssertion: require.NoError,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := webPack.clt.PostJSON(s.ctx, endpoint, tc.req)
			tc.errAssertion(t, err)
		})
	}
}

func TestUpdateSAMLIdpServiceProviderHandle(t *testing.T) {
	s := newWebSuite(t)
	webPack := s.newAuthWebPack(t, "foo")

	authClient := s.newAdminAuthClient(s.ctx, t)
	sp1, err := types.NewSAMLIdPServiceProvider(
		types.Metadata{
			Name: "sp1",
		},
		types.SAMLIdPServiceProviderSpecV1{
			ACSURL:           "https://sp1",
			EntityID:         "https://sp1",
			EntityDescriptor: newEntityDescriptor("https://sp1", "https://sp1"),
		},
	)
	require.NoError(t, err)
	err = authClient.CreateSAMLIdPServiceProvider(context.Background(), sp1)
	require.NoError(t, err)

	sp2, err := types.NewSAMLIdPServiceProvider(
		types.Metadata{
			Name: "sp2",
		},
		types.SAMLIdPServiceProviderSpecV1{
			EntityID:         "https://sp2",
			EntityDescriptor: newEntityDescriptor("https://sp2", "https://sp2"),
		},
	)
	require.NoError(t, err)
	err = authClient.CreateSAMLIdPServiceProvider(context.Background(), sp2)
	require.NoError(t, err)

	var testCases = []struct {
		name         string
		req          samlidpui.CreateSAMLIdPServiceProviderRequest
		param        string
		errAssertion require.ErrorAssertionFunc
	}{
		{
			name: "resource not available in backend",
			req: samlidpui.CreateSAMLIdPServiceProviderRequest{
				Name:             "sp_not_available",
				EntityID:         "https://sp_not_available",
				ACSURL:           "https://sp_not_available",
				EntityDescriptor: newEntityDescriptor("https://sp_not_available", "https://sp_not_available"),
				AttributeMapping: []*types.SAMLAttributeMapping{},
			},
			param: "sp_not_available",
			errAssertion: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "doesn't exist")
			},
		},
		{
			name: "rename app name",
			req: samlidpui.CreateSAMLIdPServiceProviderRequest{
				Name:             "sp2",
				EntityID:         "https://sp1",
				ACSURL:           "https://sp1",
				AttributeMapping: []*types.SAMLAttributeMapping{},
			},
			param: "sp1",
			errAssertion: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "resource renaming is not supported")
			},
		},
		{
			name: "entity ID with a value that matches with an existing entity ID in the backend",
			req: samlidpui.CreateSAMLIdPServiceProviderRequest{
				Name:             "sp1",
				EntityID:         "https://sp2",
				EntityDescriptor: newEntityDescriptor("https://sp2", "https://sp2"),
				AttributeMapping: []*types.SAMLAttributeMapping{},
			},
			param: "sp1",
			errAssertion: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "has the same entity")
			},
		},
		{
			name: "missing entity descriptor",
			req: samlidpui.CreateSAMLIdPServiceProviderRequest{
				Name:             "sp2",
				EntityID:         "https://sp2/saml",
				ACSURL:           "https://sp2/saml",
				AttributeMapping: []*types.SAMLAttributeMapping{},
			},
			param: "sp2",
			errAssertion: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "invalid entity descriptor for SAML IdP Service Provider")
			},
		},
		{
			name: "mismatch in entity ID field and entity descriptor's entity ID value",
			req: samlidpui.CreateSAMLIdPServiceProviderRequest{
				Name:             "sp2",
				EntityID:         "https://sp3/saml",
				EntityDescriptor: newEntityDescriptor("https://sp2", "https://sp2"),
				AttributeMapping: []*types.SAMLAttributeMapping{},
			},
			param: "sp2",
			errAssertion: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "parsed from the entity descriptor does not match the entity")
			},
		},
		{
			name: "update attribute mapping",
			req: samlidpui.CreateSAMLIdPServiceProviderRequest{
				Name:             "sp2",
				EntityID:         "https://sp2",
				EntityDescriptor: newEntityDescriptor("https://sp2", "https://sp2"),
				AttributeMapping: []*types.SAMLAttributeMapping{{Name: "roles", NameFormat: "", Value: "user.spec.roles"}},
			},
			param:        "sp2",
			errAssertion: require.NoError,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			endpoint := webPack.clt.Endpoint("enterprise", "samlidp", tc.param)
			_, err := webPack.clt.PutJSON(s.ctx, endpoint, tc.req)
			tc.errAssertion(t, err)
		})
	}
}

func TestDeleteSAMLIdpServiceProviderHandle(t *testing.T) {
	s := newWebSuite(t)
	webPack := s.newAuthWebPack(t, "foo")

	authClient := s.newAdminAuthClient(s.ctx, t)
	sp1, err := types.NewSAMLIdPServiceProvider(
		types.Metadata{
			Name: "sp1",
		},
		types.SAMLIdPServiceProviderSpecV1{
			ACSURL:           "https://sp1",
			EntityID:         "https://sp1",
			EntityDescriptor: newEntityDescriptor("https://sp1", "https://sp1"),
		},
	)
	require.NoError(t, err)
	err = authClient.CreateSAMLIdPServiceProvider(context.Background(), sp1)
	require.NoError(t, err)

	spFromBackend, err := authClient.GetSAMLIdPServiceProvider(context.Background(), sp1.GetName())
	require.NoError(t, err)

	endpoint := webPack.clt.Endpoint("enterprise", "samlidp", spFromBackend.GetName())
	_, err = webPack.clt.Delete(s.ctx, endpoint)
	require.NoError(t, err)

	_, err = authClient.GetSAMLIdPServiceProvider(context.Background(), sp1.GetName())
	require.ErrorContains(t, err, "doesn't exist")
}

func newEntityDescriptor(entityID, acsURL string) string {
	return fmt.Sprintf(entityDescriptor, entityID, acsURL)
}

const entityDescriptor = `<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" validUntil="2023-12-09T23:43:58.16Z" entityID="%s">
<SPSSODescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" validUntil="2023-12-09T23:43:58.16Z" protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol" AuthnRequestsSigned="false" WantAssertionsSigned="true">
  <NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified</NameIDFormat>
  <AssertionConsumerService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="%s" index="1"></AssertionConsumerService>
</SPSSODescriptor>
</EntityDescriptor>
`
