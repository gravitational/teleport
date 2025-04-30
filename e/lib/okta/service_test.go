package okta

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	oktaapi "github.com/gravitational/teleport/e/lib/okta/api"
	oktaapitest "github.com/gravitational/teleport/e/lib/okta/api/apitest"
	oktacommon "github.com/gravitational/teleport/e/lib/okta/common"
)

func TestServiceSetupWithInsufficientOauthScopes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	authorizedScopes := []string{}

	oktaClient := oktaapitest.NewClient(t, oktaapitest.ClientFuncs{
		OrgURLFunc: func(t *testing.T) string {
			return "https://okta.exmaple.com"
		},
		GetAuthorizedScopesFunc: func(t *testing.T, ctx context.Context) ([]string, error) {
			return authorizedScopes, nil
		},
	})
	creatorFn := func(context.Context, oktaapi.Config) (oktaapi.Interface, error) {
		return oktaClient, nil
	}

	ap := newTestAccessPoint(t, clockwork.NewRealClock())
	config, _ := newTestConfig(t, ap)

	createStubSAMLConnector(ctx, t, config.SyncSettings.SsoConnectorId, ap)

	// Enable all sync levels

	config.SyncSettings.SyncUsers = true
	config.SyncSettings.DisableBidirectionalSync = false
	config.SyncSettings.SyncAccessLists = true
	config.SyncSettings.DisableBidirectionalSync = false

	// Missing multiple scopes

	authorizedScopes = []string{oktaapi.ScopeUserRead}

	_, err := newWithClientCreator(ctx, config, creatorFn)
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err))
	require.ErrorContains(t, err, "checking Okta client credentials OAuth scopes\n\tscope \"okta.apps.read\" missing, scope \"okta.groups.read\" missing, scope \"okta.apps.manage\" missing, scope \"okta.groups.manage\" missing")

	// Only one scope missing

	authorizedScopes = []string{
		oktaapi.ScopeAppsRead,
		oktaapi.ScopeAppsManage,
		oktaapi.ScopeGroupsRead,
		oktaapi.ScopeGroupsManage,
	}

	_, err = newWithClientCreator(ctx, config, creatorFn)
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err))
	require.ErrorContains(t, err, "checking Okta client credentials OAuth scopes\n\tscope \"okta.users.read\" missing")

	// Add the single missing scope and expect no error

	authorizedScopes = append(
		authorizedScopes,
		oktaapi.ScopeUserRead,
	)
	_, err = newWithClientCreator(ctx, config, creatorFn)
	require.NoError(t, err)

	// For read-only mode, read-only scopes must be enough

	config.SyncSettings.DisableBidirectionalSync = true
	authorizedScopes = oktacommon.GetReadOnlyOAuthScopes()

	_, err = newWithClientCreator(ctx, config, creatorFn)
	require.NoError(t, err)
}
