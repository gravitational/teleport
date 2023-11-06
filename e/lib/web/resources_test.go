package web

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/gravitational/teleport/api/types"
	enterpriseui "github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/web/ui"
)

func TestNewResourceItemOIDC(t *testing.T) {
	const contents = `kind: oidc
metadata:
  name: oidcName
spec:
  claims_to_roles:
  - claim: roles
    roles:
    - admin
    value: teleport-user
  client_id: client-id
  client_secret: ""
  issuer_url: ""
  redirect_url: https://proxy.example.com/v1/webapi/oidc/callback
version: v3
`
	oidcConn, err := types.NewOIDCConnector("oidcName", types.OIDCConnectorSpecV3{
		ClientID: "client-id",
		ClaimsToRoles: []types.ClaimMapping{
			{
				Claim: "roles",
				Value: "teleport-user",
				Roles: []string{"admin"},
			},
		},
		RedirectURLs: []string{"https://proxy.example.com/v1/webapi/oidc/callback"},
	})
	require.NoError(t, err)

	item, err := ui.NewResourceItem(oidcConn)
	require.NoError(t, err)
	require.Equal(t, &ui.ResourceItem{
		ID:      "oidc:oidcName",
		Kind:    types.KindOIDCConnector,
		Name:    "oidcName",
		Content: contents,
	}, item)
}

func TestNewResourceItemSAML(t *testing.T) {
	const contents = `kind: saml
metadata:
  name: samlName
spec:
  acs: service
  attributes_to_roles: null
  audience: service
  cert: ""
  display: ""
  entity_descriptor: descriptor
  entity_descriptor_url: ""
  issuer: ""
  service_provider_issuer: service
  sso: ""
version: v2
`
	samlConn, err := types.NewSAMLConnector("samlName", types.SAMLConnectorSpecV2{
		AssertionConsumerService: "service",
		EntityDescriptor:         "descriptor",
	})
	require.NoError(t, err)
	item, err := ui.NewResourceItem(samlConn)
	require.NoError(t, err)
	require.Equal(t, &ui.ResourceItem{
		ID:      "saml:samlName",
		Kind:    types.KindSAMLConnector,
		Name:    "samlName",
		Content: contents,
	}, item)
}

func TestGetAuthConnectors(t *testing.T) {
	ctx := context.TODO()

	m := &mockedResourceAPIGetter{}
	m.mockGetGithubConnectors = func(ctx context.Context, withSecrets bool) ([]types.GithubConnector, error) {
		connector, err := types.NewGithubConnector("githubName", types.GithubConnectorSpecV3{
			TeamsToRoles: []types.TeamRolesMapping{
				{
					Organization: "octocats",
					Team:         "dummy",
					Roles:        []string{"dummmy"},
				},
			}})
		require.NoError(t, err)
		return []types.GithubConnector{connector}, nil
	}
	m.mockGetSAMLConnectors = func(ctx context.Context, withSecrets bool) ([]types.SAMLConnector, error) {
		connector, err := types.NewSAMLConnector("samlName", types.SAMLConnectorSpecV2{
			AssertionConsumerService: "service",
			EntityDescriptor:         "descriptor",
		})
		require.NoError(t, err)
		return []types.SAMLConnector{connector}, nil
	}
	m.mockGetOIDCConnectors = func(ctx context.Context, withSecrets bool) ([]types.OIDCConnector, error) {
		connector, err := types.NewOIDCConnector("oidcName", types.OIDCConnectorSpecV3{
			ClientID: "client-id",
			ClaimsToRoles: []types.ClaimMapping{
				{
					Claim: "roles",
					Value: "teleport-user",
					Roles: []string{"admin"},
				},
			},
			RedirectURLs: []string{"https://proxy.example.com/v1/webapi/oidc/callback"},
		})
		require.NoError(t, err)
		return []types.OIDCConnector{connector}, nil
	}

	// Test response is converted to ui objects.
	conns, err := getAuthConnectors(ctx, m)
	require.NoError(t, err)
	require.Len(t, conns, 3)
	require.Equal(t, types.KindGithub, conns[0].Kind)
	require.Equal(t, types.KindSAML, conns[1].Kind)
	require.Equal(t, types.KindOIDC, conns[2].Kind)
}

func TestSAMLConnector(t *testing.T) {
	ctx := context.Background()
	modules.SetTestModules(t, &modules.TestModules{
		TestFeatures: modules.Features{
			SAML: true,
		},
	})

	s := newWebSuite(t)
	pack := s.newAuthWebPack(t, "foo")

	expected, err := types.NewSAMLConnector("saml", types.SAMLConnectorSpecV2{
		AssertionConsumerService: "test",
		EntityDescriptor: `<?xml version="1.0" encoding="UTF-8"?>
    <md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" entityID="test">
      <md:IDPSSODescriptor WantAuthnRequestsSigned="false" protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
        <md:KeyDescriptor use="signing">
          <ds:KeyInfo xmlns:ds="http://www.w3.org/2000/09/xmldsig#">
            <ds:X509Data>
              <ds:X509Certificate></ds:X509Certificate>
            </ds:X509Data>
          </ds:KeyInfo>
        </md:KeyDescriptor>
        <md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress</md:NameIDFormat>
        <md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified</md:NameIDFormat>
        <md:SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="test" />
        <md:SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="test" />
      </md:IDPSSODescriptor>
    </md:EntityDescriptor>`,
		Display: "SAML",
		AttributesToRoles: []types.AttributeMapping{
			{
				Name:  "test",
				Value: "test",
				Roles: []string{"default-implicit-role"},
			},
		},
	})
	require.NoError(t, err, "creating initial connector resource")

	createPayload := func(connector types.SAMLConnector) ui.ResourceItem {
		raw, err := services.MarshalSAMLConnector(connector, services.PreserveResourceID())
		require.NoError(t, err, "marshaling connector")

		return ui.ResourceItem{
			Kind:    types.KindSAMLConnector,
			Name:    connector.GetName(),
			Content: string(raw),
		}
	}

	unmarshalResponse := func(resp []byte) types.SAMLConnector {
		var item ui.ResourceItem
		require.NoError(t, json.Unmarshal(resp, &item), "response from server contained an invalid resource item")

		var conn types.SAMLConnectorV2
		require.NoError(t, yaml.Unmarshal([]byte(item.Content), &conn), "resource item content was not an oidc connector")
		return &conn
	}

	// Create the initial connector.
	resp, err := pack.clt.PostJSON(ctx, pack.clt.Endpoint("enterprise", "saml"), createPayload(expected))
	require.NoError(t, err, "expected creating the initial connector to succeed")
	require.Equal(t, http.StatusOK, resp.Code(), "unexpected status code creating connector")

	created := unmarshalResponse(resp.Bytes())

	// Validate that creating the connector again fails.
	resp, err = pack.clt.PostJSON(ctx, pack.clt.Endpoint("enterprise", "saml"), createPayload(expected))
	assert.Error(t, err, "expected an error creating a duplicate connector")
	assert.True(t, trace.IsAlreadyExists(err), "expected an already exists error got %T", err)
	assert.Equal(t, http.StatusConflict, resp.Code(), "unexpected status code creating duplicate connector")

	// Update the connector.
	created.SetDisplay("test")
	resp, err = pack.clt.PutJSON(ctx, pack.clt.Endpoint("enterprise", "saml", expected.GetName()), createPayload(created))
	require.NoError(t, err, "unexpected error updating the connector")
	require.Equal(t, http.StatusOK, resp.Code(), "unexpected status code updating the connector")

	updated := unmarshalResponse(resp.Bytes())

	require.Empty(t, cmp.Diff(created, updated, cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision", "Namespace"),
		cmpopts.IgnoreFields(types.SAMLConnectorSpecV2{}, "Display"),
	))
	require.NotEqual(t, expected.GetDisplay(), updated.GetDisplay(), "expected update to modify the display name")
	require.Equal(t, "test", updated.GetDisplay(), "display name should have been updated to test. got %s", updated.GetDisplay())

	// Validate that a stale revision prevents updates.
	resp, err = pack.clt.PutJSON(ctx, pack.clt.Endpoint("enterprise", "saml", expected.GetName()), createPayload(expected))
	assert.Error(t, err, "expected an error updating a connector with a stale revision")
	assert.True(t, trace.IsCompareFailed(err), "expected a compare failed error got %T", err)
	assert.Equal(t, http.StatusPreconditionFailed, resp.Code(), "unexpected status code updating the connector")

	// Validate that renaming the connector prevents updates.
	updated.SetName(uuid.NewString())
	resp, err = pack.clt.PutJSON(ctx, pack.clt.Endpoint("enterprise", "saml", expected.GetName()), createPayload(updated))
	assert.Error(t, err, "expected and error when renaming a connector")
	assert.True(t, trace.IsBadParameter(err), "expected a bad parameter error got %T", err)
	assert.Equal(t, http.StatusBadRequest, resp.Code(), "unexpected status code updating the connector")

	// Validate that updating a nonexistent connector fails.
	updated.SetName(uuid.NewString())
	resp, err = pack.clt.PutJSON(ctx, pack.clt.Endpoint("enterprise", "saml", updated.GetName()), createPayload(updated))
	assert.Error(t, err, "expected updating a nonexistent connector to fail")
	assert.True(t, trace.IsCompareFailed(err), "expected a compare failed error got %T", err)
	assert.Equal(t, http.StatusPreconditionFailed, resp.Code(), "unexpected status code updating the connector")

	// Validate that the connector can be deleted
	_, err = pack.clt.Delete(ctx, pack.clt.Endpoint("enterprise", "saml", expected.GetName()))
	require.NoError(t, err, "unexpected error deleting connector")

	resp, err = pack.clt.Get(ctx, pack.clt.Endpoint("enterprise", "authconnectors"), nil)
	assert.NoError(t, err, "unexpected error listing oidc connectors")

	var item []ui.ResourceItem
	require.NoError(t, json.Unmarshal(resp.Bytes(), &item), "invalid resource item received")

	assert.Empty(t, item)
	assert.Equal(t, http.StatusOK, resp.Code(), "unexpected status code getting connectors")
}

func TestUpsertSAMLIdpServiceProvider(t *testing.T) {
	m := &mockedResourceAPIGetter{}

	existingServiceProviders := make(map[string]types.SAMLIdPServiceProvider)
	m.mockCreateSAMLIdPServiceProvider = func(ctx context.Context, sp types.SAMLIdPServiceProvider) error {
		existingServiceProviders[sp.GetName()] = sp
		return nil
	}
	m.mockGetSAMLIdPServiceProvider = func(ctx context.Context, name string) (types.SAMLIdPServiceProvider, error) {
		sp, ok := existingServiceProviders[name]
		if ok {
			return sp, nil
		}
		return nil, trace.NotFound("")
	}

	// Test bad request.
	invalidRequest := &enterpriseui.CreateSAMLIdPServiceProviderRequest{Name: "app_saml", EntityDescriptor: "<invalid xml"}

	sp, err := upsertSAMLIdPServiceProvider(context.Background(), m, *invalidRequest, "", httprouter.Params{})
	require.Nil(t, sp)
	require.Error(t, err)

	goodRequest := &enterpriseui.CreateSAMLIdPServiceProviderRequest{Name: "app_saml", EntityDescriptor: `<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" validUntil="2023-06-03T09:53:47.739Z" entityID="https://test.com/saml/metadata">
	<SPSSODescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" validUntil="2023-06-03T09:53:47.738823Z" protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol" AuthnRequestsSigned="false" WantAssertionsSigned="true">
	 <KeyDescriptor use="encryption">
		<KeyInfo xmlns="http://www.w3.org/2000/09/xmldsig#">
		 <X509Data xmlns="http://www.w3.org/2000/09/xmldsig#">
			<X509Certificate xmlns="http://www.w3.org/2000/09/xmldsig#">abcdefg</X509Certificate>
		 </X509Data>
		</KeyInfo>
		<EncryptionMethod Algorithm="http://www.w3.org/2001/04/xmlenc#aes128-cbc"></EncryptionMethod>
		<EncryptionMethod Algorithm="http://www.w3.org/2001/04/xmlenc#aes192-cbc"></EncryptionMethod>
		<EncryptionMethod Algorithm="http://www.w3.org/2001/04/xmlenc#aes256-cbc"></EncryptionMethod>
		<EncryptionMethod Algorithm="http://www.w3.org/2001/04/xmlenc#rsa-oaep-mgf1p"></EncryptionMethod>
	 </KeyDescriptor>
	 <NameIDFormat>urn:oasis:names:tc:SAML:2.0:nameid-format:transient</NameIDFormat>
	 <AssertionConsumerService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="https://test.com/saml/acs" index="1"></AssertionConsumerService>
	 <AssertionConsumerService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Artifact" Location="https://test.com/saml/acs" index="2"></AssertionConsumerService>
	</SPSSODescriptor>
 </EntityDescriptor>`}

	// Creating new service provider succeeds.
	sp, err = upsertSAMLIdPServiceProvider(context.Background(), m, *goodRequest, "POST", httprouter.Params{})
	require.NoError(t, err)
	require.Contains(t, sp.Content, "entity_id: https://test.com/saml/metadata")

	// Creating existing service provider fails.
	sp, err = upsertSAMLIdPServiceProvider(context.Background(), m, *goodRequest, "POST", httprouter.Params{})
	require.Nil(t, sp)
	require.Error(t, err)
	require.True(t, trace.IsAlreadyExists(err))
}

type mockedResourceAPIGetter struct {
	mockGetGithubConnectors          func(ctx context.Context, withSecrets bool) ([]types.GithubConnector, error)
	mockUpsertSAMLConnector          func(ctx context.Context, connector types.SAMLConnector) error
	mockGetSAMLConnector             func(ctx context.Context, id string, withSecrets bool) (types.SAMLConnector, error)
	mockGetSAMLConnectors            func(ctx context.Context, withSecrets bool) ([]types.SAMLConnector, error)
	mockUpsertOIDCConnector          func(ctx context.Context, connector types.OIDCConnector) error
	mockGetOIDCConnector             func(ctx context.Context, id string, withSecrets bool) (types.OIDCConnector, error)
	mockGetOIDCConnectors            func(ctx context.Context, withSecrets bool) ([]types.OIDCConnector, error)
	mockCreateSAMLIdPServiceProvider func(ctx context.Context, sp types.SAMLIdPServiceProvider) error
	mockGetSAMLIdPServiceProvider    func(ctx context.Context, name string) (types.SAMLIdPServiceProvider, error)
}

func (m *mockedResourceAPIGetter) GetGithubConnectors(ctx context.Context, withSecrets bool) ([]types.GithubConnector, error) {
	if m.mockGetGithubConnectors != nil {
		return m.mockGetGithubConnectors(ctx, false)
	}

	return nil, trace.NotImplemented("mockGetGithubConnectors not implemented")
}

func (m *mockedResourceAPIGetter) UpsertSAMLConnector(ctx context.Context, connector types.SAMLConnector) error {
	if m.mockUpsertSAMLConnector != nil {
		return m.mockUpsertSAMLConnector(ctx, connector)
	}

	return trace.NotImplemented("mockUpsertSAMLConnector not implemented")
}

func (m *mockedResourceAPIGetter) GetSAMLConnector(ctx context.Context, id string, withSecrets bool) (types.SAMLConnector, error) {
	if m.mockGetSAMLConnector != nil {
		return m.mockGetSAMLConnector(ctx, id, withSecrets)
	}

	return nil, trace.NotImplemented("mockGetSAMLConnector not implemented")
}

func (m *mockedResourceAPIGetter) GetSAMLConnectors(ctx context.Context, withSecrets bool) ([]types.SAMLConnector, error) {
	if m.mockGetSAMLConnectors != nil {
		return m.mockGetSAMLConnectors(ctx, withSecrets)
	}

	return nil, trace.NotImplemented("mockGetSAMLConnectors not implemented")
}

func (m *mockedResourceAPIGetter) UpsertOIDCConnector(ctx context.Context, connector types.OIDCConnector) error {
	if m.mockUpsertOIDCConnector != nil {
		return m.mockUpsertOIDCConnector(ctx, connector)
	}

	return trace.NotImplemented("mockUpsertOIDCConnector not implemented")
}

func (m *mockedResourceAPIGetter) GetOIDCConnector(ctx context.Context, id string, withSecrets bool) (types.OIDCConnector, error) {
	if m.mockGetOIDCConnector != nil {
		return m.mockGetOIDCConnector(ctx, id, withSecrets)
	}

	return nil, trace.NotImplemented("mockGetOIDCConnector not implemented")
}

func (m *mockedResourceAPIGetter) GetOIDCConnectors(ctx context.Context, withSecrets bool) ([]types.OIDCConnector, error) {
	if m.mockGetOIDCConnectors != nil {
		return m.mockGetOIDCConnectors(ctx, withSecrets)
	}

	return nil, trace.NotImplemented("mockGetOIDCConnectors not implemented")
}

func (m *mockedResourceAPIGetter) CreateSAMLIdPServiceProvider(ctx context.Context, sp types.SAMLIdPServiceProvider) error {
	if m.mockCreateSAMLIdPServiceProvider != nil {
		return m.mockCreateSAMLIdPServiceProvider(ctx, sp)
	}

	return trace.NotImplemented("mockCreateSAMlIdPServiceProvider not implemented")
}

func (m *mockedResourceAPIGetter) GetSAMLIdPServiceProvider(ctx context.Context, name string) (types.SAMLIdPServiceProvider, error) {
	if m.mockGetSAMLIdPServiceProvider != nil {
		return m.mockGetSAMLIdPServiceProvider(ctx, name)
	}

	return nil, trace.NotImplemented("mockGetSAMlIdPServiceProvider not implemented")
}

func TestOIDCConnector(t *testing.T) {
	ctx := context.Background()
	modules.SetTestModules(t, &modules.TestModules{
		TestFeatures: modules.Features{
			OIDC: true,
		},
	})

	s := newWebSuite(t)
	pack := s.newAuthWebPack(t, "foo")

	expected, err := types.NewOIDCConnector("github", types.OIDCConnectorSpecV3{
		ClientID:     "12345",
		ClientSecret: "678910",
		RedirectURLs: []string{"https://proxy.example.com/v1/webapi/oidc/callback"},
		Display:      "Github",
		ClaimsToRoles: []types.ClaimMapping{
			{
				Claim: "test",
				Value: "test",
				Roles: []string{"access", "editor", "auditor"},
			},
		},
	})
	require.NoError(t, err, "creating initial connector resource")

	createPayload := func(connector types.OIDCConnector) ui.ResourceItem {
		raw, err := services.MarshalOIDCConnector(connector, services.PreserveResourceID())
		require.NoError(t, err, "marshaling connector")

		return ui.ResourceItem{
			Kind:    types.KindOIDCConnector,
			Name:    connector.GetName(),
			Content: string(raw),
		}
	}

	unmarshalResponse := func(resp []byte) types.OIDCConnector {
		var item ui.ResourceItem
		require.NoError(t, json.Unmarshal(resp, &item), "response from server contained an invalid resource item")

		var conn types.OIDCConnectorV3
		require.NoError(t, yaml.Unmarshal([]byte(item.Content), &conn), "resource item content was not an oidc connector")
		return &conn
	}

	// Create the initial connector.
	resp, err := pack.clt.PostJSON(ctx, pack.clt.Endpoint("enterprise", "oidc"), createPayload(expected))
	require.NoError(t, err, "expected creating the initial connector to succeed")
	require.Equal(t, http.StatusOK, resp.Code(), "unexpected status code creating connector")

	created := unmarshalResponse(resp.Bytes())

	// Validate that creating the connector again fails.
	resp, err = pack.clt.PostJSON(ctx, pack.clt.Endpoint("enterprise", "oidc"), createPayload(expected))
	assert.Error(t, err, "expected an error creating a duplicate connector")
	assert.True(t, trace.IsAlreadyExists(err), "expected an already exists error got %T", err)
	assert.Equal(t, http.StatusConflict, resp.Code(), "unexpected status code creating duplicate connector")

	// Update the connector.
	created.SetDisplay("test")
	resp, err = pack.clt.PutJSON(ctx, pack.clt.Endpoint("enterprise", "oidc", expected.GetName()), createPayload(created))
	require.NoError(t, err, "unexpected error updating the connector")
	require.Equal(t, http.StatusOK, resp.Code(), "unexpected status code updating the connector")

	updated := unmarshalResponse(resp.Bytes())

	require.Empty(t, cmp.Diff(created, updated, cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision", "Namespace"),
		cmpopts.IgnoreFields(types.GithubConnectorSpecV3{}, "Display", "ClientSecret"),
	))
	require.NotEqual(t, expected.GetDisplay(), updated.GetDisplay(), "expected update to modify the display name")
	require.Equal(t, "test", updated.GetDisplay(), "display name should have been updated to test. got %s", updated.GetDisplay())

	// Validate that a stale revision prevents updates.
	resp, err = pack.clt.PutJSON(ctx, pack.clt.Endpoint("enterprise", "oidc", expected.GetName()), createPayload(expected))
	assert.Error(t, err, "expected an error updating a connector with a stale revision")
	assert.True(t, trace.IsCompareFailed(err), "expected a compare failed error got %T", err)
	assert.Equal(t, http.StatusPreconditionFailed, resp.Code(), "unexpected status code updating the connector")

	// Validate that renaming the connector prevents updates.
	updated.SetName(uuid.NewString())
	resp, err = pack.clt.PutJSON(ctx, pack.clt.Endpoint("enterprise", "oidc", expected.GetName()), createPayload(updated))
	assert.Error(t, err, "expected and error when renaming a connector")
	assert.True(t, trace.IsBadParameter(err), "expected a bad parameter error got %T", err)
	assert.Equal(t, http.StatusBadRequest, resp.Code(), "unexpected status code updating the connector")

	// Validate that updating a nonexistent connector fails.
	updated.SetName(uuid.NewString())
	resp, err = pack.clt.PutJSON(ctx, pack.clt.Endpoint("enterprise", "oidc", updated.GetName()), createPayload(updated))
	assert.Error(t, err, "expected updating a nonexistent connector to fail")
	assert.True(t, trace.IsCompareFailed(err), "expected a compare failed error got %T", err)
	assert.Equal(t, http.StatusPreconditionFailed, resp.Code(), "unexpected status code updating the connector")

	// Validate that the connector can be deleted
	_, err = pack.clt.Delete(ctx, pack.clt.Endpoint("enterprise", "oidc", expected.GetName()))
	require.NoError(t, err, "unexpected error deleting connector")

	resp, err = pack.clt.Get(ctx, pack.clt.Endpoint("enterprise", "authconnectors"), nil)
	assert.NoError(t, err, "unexpected error listing oidc connectors")

	var item []ui.ResourceItem
	require.NoError(t, json.Unmarshal(resp.Bytes(), &item), "invalid resource item received")

	assert.Empty(t, item)
	assert.Equal(t, http.StatusOK, resp.Code(), "unexpected status code getting connectors")
}
