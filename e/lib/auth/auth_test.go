package auth

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	etypes "github.com/gravitational/teleport/e/api/types"
	_ "github.com/gravitational/teleport/e/tool/modules" // registers the enterprise github connector marshaling
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/storage"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/observability/tracing"
	"github.com/gravitational/teleport/lib/services"
)

func resourceFromYAML(t *testing.T, value string) types.Resource {
	t.Helper()

	ur := &services.UnknownResource{}
	err := kyaml.NewYAMLToJSONDecoder(strings.NewReader(value)).Decode(ur)
	require.NoError(t, err)

	resource, err := services.UnmarshalResource(ur.Kind, ur.Raw)
	require.NoError(t, err)
	return resource
}

const (
	githubConnectorYAML = `kind: github
metadata:
  name: github
spec:
  client_id: "12345"
  client_secret: "678910"
  display: Github
  redirect_url: https://proxy.example.com/v1/webapi/github/callback
  teams_to_roles:
  - organization: acme
    roles:
    - access
    - editor
    - auditor
    team: users
version: v3`
	oidcConnectorYAML = `kind: oidc
version: v3
metadata:
  name: oidc
spec:
  redirect_url: "https://proxy.example.com/v1/webapi/oidc/callback"
  client_id: "12345"
  client_secret: "678910"
  display: OIDC
  scope: [roles]
  claims_to_roles:
    - {claim: "test", value: "test", roles: ["access", "editor", "auditor"]}`
	samlConnectorYAML = `kind: saml
version: v2
metadata:
  name: saml
spec:
  acs: test
  audience: test
  issuer: test
  sso: https://example.com
  service_provider_issuer: test
  display: SAML
  attributes_to_roles:
  - name: test
    roles:
    - access
    value: test
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
        <md:SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="https://example.com" />
        <md:SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://example.com" />
      </md:IDPSSODescriptor>
    </md:EntityDescriptor>` + "\n"
)

func setupConfig(t *testing.T) auth.InitConfig {
	tempDir := t.TempDir()

	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)

	processStorage, err := storage.NewProcessStorage(
		context.Background(),
		filepath.Join(tempDir, teleport.ComponentProcess),
	)
	require.NoError(t, err)

	clusterName, err := services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: "me.localhost",
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		bk.Close()
		processStorage.Close()
	})

	authority, err := testauthority.NewKeygen(modules.BuildEnterprise, time.Now)
	require.NoError(t, err)

	return auth.InitConfig{
		DataDir:                 tempDir,
		HostUUID:                "00000000-0000-0000-0000-000000000000",
		NodeName:                "foo",
		Backend:                 bk,
		VersionStorage:          processStorage,
		Authority:               authority,
		ClusterAuditConfig:      types.DefaultClusterAuditConfig(),
		ClusterNetworkingConfig: types.DefaultClusterNetworkingConfig(),
		SessionRecordingConfig:  types.DefaultSessionRecordingConfig(),
		ClusterName:             clusterName,
		StaticTokens:            types.DefaultStaticTokens(),
		AuthPreference:          types.DefaultAuthPreference(),
		SkipPeriodicOperations:  true,
		Tracer:                  tracing.NoopTracer(teleport.ComponentAuth),
	}
}

func TestBootstrap(t *testing.T) {
	t.Parallel()

	github := resourceFromYAML(t, githubConnectorYAML).(types.GithubConnector)
	oidc := resourceFromYAML(t, oidcConnectorYAML).(types.OIDCConnector)
	saml := resourceFromYAML(t, samlConnectorYAML).(types.SAMLConnector)

	ur := &services.UnknownResource{}
	err := kyaml.NewYAMLToJSONDecoder(strings.NewReader(githubConnectorYAML)).Decode(ur)
	require.NoError(t, err)

	githubEnt, err := etypes.UnmarshalGithubConnector(ur.Raw)
	require.NoError(t, err)

	tests := []struct {
		name         string
		modifyConfig func(*auth.InitConfig)
		assertError  require.ErrorAssertionFunc
	}{

		{
			name: "Apply OSS GitHubConnector",
			modifyConfig: func(cfg *auth.InitConfig) {
				cfg.BootstrapResources = append(cfg.BootstrapResources, github)
			},
			assertError: require.NoError,
		},
		{
			name: "Apply Enterprise GitHubConnector",
			modifyConfig: func(cfg *auth.InitConfig) {
				cfg.BootstrapResources = append(cfg.BootstrapResources, githubEnt)
			},
			assertError: require.NoError,
		},
		{
			name: "Apply OIDCConnector",
			modifyConfig: func(cfg *auth.InitConfig) {
				cfg.BootstrapResources = append(cfg.BootstrapResources, oidc)
			},
			assertError: require.NoError,
		},
		{
			name: "Apply SAMLConnector",
			modifyConfig: func(cfg *auth.InitConfig) {
				cfg.BootstrapResources = append(cfg.BootstrapResources, saml)
			},
			assertError: require.NoError,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := setupConfig(t)
			test.modifyConfig(&cfg)

			_, err := auth.Init(context.Background(), cfg)
			test.assertError(t, err)
		})
	}

}
