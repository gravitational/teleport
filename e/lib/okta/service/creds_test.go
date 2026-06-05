package oktaservice

import (
	"context"
	"testing"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	oktav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
	oktaapi "github.com/gravitational/teleport/e/lib/okta/api"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/okta/oktatest"
	"github.com/gravitational/teleport/lib/services/local"
)

// Verifies that if plugin credentials are used then the Okta org URL is also taken from plugin. To
// mitigate the potential issue of capturing exiting Okta API token from the plugin by providing a
// malicious org URL.
func Test_createOktaClient_orgUrl(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	svc := setupServiceForRealOktaClientCreation(t)

	maliciousOrgUrl := "https://evil.okta.example.com"
	goodOrgUrl := "https://good.okta.example.com"

	plugin := oktatest.NewPlugin(t, oktatest.WithOrgURL(goodOrgUrl))
	oktatest.UpsertPlugin(t, svc.pluginBackend, plugin)

	staticCreds := oktatest.NewPluginStaticCredentials(t, plugin,
		oktatest.WithAPIToken("test-api-token-secret"),
	)
	oktatest.UpsertPluginStaticCredentials(t, svc.credsBackend, staticCreds)

	req := oktav1.GetAppsRequest_builder{
		ApiCredentials:      nil,             // no API credentials
		OktaOrganizationUrl: maliciousOrgUrl, // but malicious org URL trying to intercept the credentials from the plugin
	}.Build()
	client, err := svc.createOktaClient(ctx, req, nil)
	require.NoError(t, err)
	require.NotEqual(t, maliciousOrgUrl, client.GetOrgUrl())
	require.Equal(t, goodOrgUrl, client.GetOrgUrl())
}

func setupServiceForRealOktaClientCreation(t *testing.T) *Service {
	t.Helper()

	clock := clockwork.NewRealClock()
	backend, err := memory.New(memory.Config{
		Clock: clock,
	})
	require.NoError(t, err, "memory.New")

	pluginsSrv := local.NewPluginsService(backend)
	pluginStaticCredentialsSvc, err := local.NewPluginStaticCredentialsService(backend)
	require.NoError(t, err, "local.NewPluginStaticCredentialsService")

	return &Service{
		pluginBackend:       pluginsSrv,
		credsBackend:        pluginStaticCredentialsSvc,
		apiClientProviderFn: oktaapi.New,
	}
}
