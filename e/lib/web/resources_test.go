package web

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
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
	require.Nil(t, err)
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
	require.Nil(t, err)
	require.Len(t, conns, 3)
	require.Equal(t, conns[0].Kind, types.KindGithub)
	require.Equal(t, conns[1].Kind, types.KindSAML)
	require.Equal(t, conns[2].Kind, types.KindOIDC)
}

func TestUpsertSAMLConnector(t *testing.T) {
	m := &mockedResourceAPIGetter{}

	existingConnectors := make(map[string]types.SAMLConnector)
	m.mockUpsertSAMLConnector = func(ctx context.Context, connector types.SAMLConnector) error {
		existingConnectors[connector.GetName()] = connector
		return nil
	}
	m.mockGetSAMLConnector = func(ctx context.Context, name string, withSecrets bool) (types.SAMLConnector, error) {
		connector, ok := existingConnectors[name]
		if ok {
			return connector, nil
		}
		return nil, trace.NotFound("")
	}

	// Test bad request kind.
	invalidKind := `kind: invalid-kind
metadata:
  name: test`
	connector, err := upsertSAMLConnector(context.Background(), m, invalidKind, "", httprouter.Params{})
	require.Nil(t, connector)
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err))
	require.Contains(t, err.Error(), "kind")

	goodContent := `kind: saml
version: v2
metadata:
  name: test-goodcontent
spec:
  acs: test
  attributes_to_roles:
  - name: foo
    roles:
    - access
    value: bar
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

	// Updating non-existing connector fails.
	connector, err = upsertSAMLConnector(context.Background(), m, goodContent, "PUT", httprouter.Params{httprouter.Param{Key: "name", Value: "test-goodcontent"}})
	require.Nil(t, connector)
	require.Error(t, err)
	require.True(t, trace.IsNotFound(err))

	// Creating non-existing connector succeeds.
	connector, err = upsertSAMLConnector(context.Background(), m, goodContent, "POST", httprouter.Params{})
	require.NoError(t, err)
	require.Contains(t, connector.Content, "name: test-goodcontent")

	// Creating existing connector fails.
	connector, err = upsertSAMLConnector(context.Background(), m, goodContent, "POST", httprouter.Params{})
	require.Nil(t, connector)
	require.Error(t, err)
	require.True(t, trace.IsAlreadyExists(err))

	// Updating existing connector succeeds.
	connector, err = upsertSAMLConnector(context.Background(), m, goodContent, "PUT", httprouter.Params{httprouter.Param{Key: "name", Value: "test-goodcontent"}})
	require.NoError(t, err)
	require.Contains(t, connector.Content, "name: test-goodcontent")

	// Renaming existing connector fails.
	goodContentRenamed := strings.ReplaceAll(goodContent, "test-goodcontent", "test-goodcontent-new-name")
	connector, err = upsertSAMLConnector(context.Background(), m, goodContentRenamed, "PUT", httprouter.Params{httprouter.Param{Key: "name", Value: "test-goodcontent"}})
	require.Nil(t, connector)
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err))
}

func TestUpsertOIDCConnector(t *testing.T) {
	m := &mockedResourceAPIGetter{}

	existingConnectors := make(map[string]types.OIDCConnector)
	m.mockUpsertOIDCConnector = func(ctx context.Context, connector types.OIDCConnector) error {
		existingConnectors[connector.GetName()] = connector
		return nil
	}
	m.mockGetOIDCConnector = func(ctx context.Context, name string, withSecrets bool) (types.OIDCConnector, error) {
		connector, ok := existingConnectors[name]
		if ok {
			return connector, nil
		}
		return nil, trace.NotFound("")
	}

	// Test bad request kind.
	invalidKind := `kind: invalid-kind
metadata:
  name: test`
	connector, err := upsertOIDCConnector(context.Background(), m, invalidKind, "", httprouter.Params{})
	require.Nil(t, connector)
	require.Error(t, err)
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

	// Updating non-existing connector fails.
	connector, err = upsertOIDCConnector(context.Background(), m, goodContent, "PUT", httprouter.Params{httprouter.Param{Key: "name", Value: "test-goodcontent"}})
	require.Nil(t, connector)
	require.Error(t, err)
	fmt.Printf("ERROR %v\n", err)
	require.True(t, trace.IsNotFound(err))

	// Creating non-existing connector succeeds.
	connector, err = upsertOIDCConnector(context.Background(), m, goodContent, "POST", httprouter.Params{})
	require.NoError(t, err)
	require.Contains(t, connector.Content, "name: test-goodcontent")

	// Creating existing connector fails.
	connector, err = upsertOIDCConnector(context.Background(), m, goodContent, "POST", httprouter.Params{})
	require.Nil(t, connector)
	require.Error(t, err)
	require.True(t, trace.IsAlreadyExists(err))

	// Updating existing connector succeeds.
	connector, err = upsertOIDCConnector(context.Background(), m, goodContent, "PUT", httprouter.Params{httprouter.Param{Key: "name", Value: "test-goodcontent"}})
	require.NoError(t, err)
	require.Contains(t, connector.Content, "name: test-goodcontent")

	// Renaming existing connector fails.
	goodContentRenamed := strings.ReplaceAll(goodContent, "test-goodcontent", "test-goodcontent-new-name")
	connector, err = upsertOIDCConnector(context.Background(), m, goodContentRenamed, "PUT", httprouter.Params{httprouter.Param{Key: "name", Value: "test-goodcontent"}})
	require.Nil(t, connector)
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err))
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
