package web

import (
	"context"
	"testing"

	"github.com/gravitational/teleport/api/v7/types"
	"github.com/gravitational/teleport/lib/web/ui"

	"github.com/gravitational/trace"

	"github.com/stretchr/testify/require"
)

func TestNewResourceItemOIDC(t *testing.T) {
	const contents = `kind: oidc
metadata:
  name: oidcName
spec:
  client_id: client-id
  client_secret: ""
  issuer_url: ""
  redirect_url: ""
version: v2
`
	oidcConn, err := types.NewOIDCConnector("oidcName", types.OIDCConnectorSpecV2{ClientID: "client-id"})
	require.NoError(t, err)

	item, err := ui.NewResourceItem(oidcConn)
	require.Nil(t, err)
	require.Equal(t, item, &ui.ResourceItem{
		ID:      "oidc:oidcName",
		Kind:    types.KindOIDCConnector,
		Name:    "oidcName",
		Content: contents,
	})
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
	require.Nil(t, err)
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
		connector, err := types.NewGithubConnector("githubName", types.GithubConnectorSpecV3{})
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
		connector, err := types.NewOIDCConnector("oidcName", types.OIDCConnectorSpecV2{ClientID: "client-id"})
		require.NoError(t, err)
		return []types.OIDCConnector{connector}, nil
	}

	// Test response is converted to ui objects.
	conns, err := getAuthConnectors(ctx, m)
	require.Nil(t, err)
	require.Len(t, conns, 3)
	require.Equal(t, conns[0].Kind, types.KindGithub)
	require.Equal(t, conns[1].Kind, types.KindSAML)
	require.Equal(t, conns[2].Kind, types.KindOIDC)
}

func TestUpsertSAMLConnector(t *testing.T) {
	m := &mockedResourceAPIGetter{}
	m.mockUpsertSAMLConnector = func(ctx context.Context, connector types.SAMLConnector) error {
		return nil
	}
	m.mockGetSAMLConnector = func(ctx context.Context, id string, withSecrets bool) (types.SAMLConnector, error) {
		return nil, trace.NotFound("")
	}

	goodContent := `kind: saml
version: v2
metadata:
  name: test-goodcontent
spec:
  acs: test
  entity_descriptor: |
    <?xml version="1.0" encoding="UTF-8"?>
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
    </md:EntityDescriptor>`

	// Test POST (create) connector.
	connector, err := upsertSAMLConnector(context.Background(), m, goodContent, "POST")
	require.Nil(t, err)
	require.Contains(t, connector.Content, "name: test-goodcontent")
}

func TestUpsertOIDCConnector(t *testing.T) {
	m := &mockedResourceAPIGetter{}
	m.mockUpsertOIDCConnector = func(ctx context.Context, connector types.OIDCConnector) error {
		return nil
	}
	m.mockGetOIDCConnector = func(ctx context.Context, id string, withSecrets bool) (types.OIDCConnector, error) {
		return nil, trace.NotFound("")
	}

	// Test bad request kind.
	invalidKind := `kind: invalid-kind
metadata:
  name: test`
	conn, err := upsertOIDCConnector(context.Background(), m, invalidKind, "")
	require.Nil(t, conn)
	require.True(t, trace.IsBadParameter(err))
	require.Contains(t, err.Error(), "kind")

	goodContent := `kind: oidc
version: v2
metadata:
  name: test-goodcontent
spec:
  redirect_url: "https://<cluster-url>/v1/webapi/oidc/callback"
  client_id: <client id>
  display: Google
  client_secret: <client secret>
  issuer_url: https://<issuer-url>
  scope: [<scope value>]
  claims_to_roles:
    - {claim: "hd", value: "example.com", roles: ["admin"]}`

	// Test POST (create) connector.
	connector, err := upsertOIDCConnector(context.Background(), m, goodContent, "POST")
	require.Nil(t, err)
	require.Contains(t, connector.Content, "name: test-goodcontent")
}

type mockedResourceAPIGetter struct {
	mockGetGithubConnectors func(ctx context.Context, withSecrets bool) ([]types.GithubConnector, error)
	mockUpsertSAMLConnector func(ctx context.Context, connector types.SAMLConnector) error
	mockGetSAMLConnector    func(ctx context.Context, id string, withSecrets bool) (types.SAMLConnector, error)
	mockGetSAMLConnectors   func(ctx context.Context, withSecrets bool) ([]types.SAMLConnector, error)
	mockUpsertOIDCConnector func(ctx context.Context, connector types.OIDCConnector) error
	mockGetOIDCConnector    func(ctx context.Context, id string, withSecrets bool) (types.OIDCConnector, error)
	mockGetOIDCConnectors   func(ctx context.Context, withSecrets bool) ([]types.OIDCConnector, error)
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
