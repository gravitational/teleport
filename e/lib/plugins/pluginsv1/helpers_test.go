package pluginsv1

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/tlsca"
)

type suite struct {
	authorizer                     *fakeAuthorizer
	pluginService                  services.Plugins
	pluginStaticCredentialsService services.PluginStaticCredentials
	svc                            *Service
}

// createPlugin creates a simple plugin directly in the underlying backend service.
// This is intended for setting up initial data.
func (s *suite) createPlugin(t *testing.T, name string) {
	plugin := &types.PluginV1{
		Metadata: types.Metadata{
			Name: name,
		},
		Spec: types.PluginSpecV1{
			Settings: &types.PluginSpecV1_SlackAccessPlugin{
				SlackAccessPlugin: &types.PluginSlackAccessSettings{
					FallbackChannel: "access-requests",
				},
			},
		},
		Credentials: &types.PluginCredentialsV1{
			Credentials: &types.PluginCredentialsV1_Oauth2AccessToken{
				Oauth2AccessToken: &types.PluginOAuth2AccessTokenCredentials{
					AccessToken:  "foo",
					RefreshToken: "bar",
					Expires:      time.Now().Add(6 * time.Hour),
				},
			},
		},
	}

	err := s.pluginService.CreatePlugin(context.Background(), plugin)
	require.NoError(t, err)
}

func (s *suite) setRules(rules []types.Rule) {
	s.authorizer.checker.rules = rules
}

func (s *suite) setRoles(roles []string) {
	s.authorizer.checker.roles = roles
}

func createSuite(t *testing.T) *suite {
	return createSuiteWithModules(t, defaultPluginTestModules())
}

func createSuiteWithModules(t *testing.T, mod modules.Modules) *suite {
	mem, err := memory.New(memory.Config{
		Clock: clockwork.NewFakeClock(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, mem.Close()) })

	authorizer := &fakeAuthorizer{checker: &fakeChecker{}}

	authServer, err := authtest.NewAuthServer(authtest.AuthServerConfig{
		Dir:     t.TempDir(),
		Clock:   clockwork.NewFakeClock(),
		Modules: mod,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, authServer.Close()) })

	pluginService := local.NewPluginsService(authServer.Backend)
	pluginStaticCredentialsService, err := local.NewPluginStaticCredentialsService(authServer.Backend)
	require.NoError(t, err)

	serviceUnderTest, err := NewService(ServiceConfig{
		Authorizer:                     authorizer,
		AuthServer:                     authServer.AuthServer,
		PluginService:                  pluginService,
		PluginStaticCredentialsService: pluginStaticCredentialsService,
		HostedPluginConfig: servicecfg.HostedPluginsConfig{
			OAuthProviders: servicecfg.PluginOAuthProviders{
				SlackCredentials: &servicecfg.OAuthClientCredentials{
					ClientID: "123456", ClientSecret: "bar",
				}}},
		KeyStoreManager: authServer.AuthServer.GetKeyStore(),
		Modules:         mod,
	})
	require.NoError(t, err)

	return &suite{
		authorizer:                     authorizer,
		pluginService:                  pluginService,
		pluginStaticCredentialsService: pluginStaticCredentialsService,
		svc:                            serviceUnderTest,
	}
}

func defaultPluginTestModules() modules.Modules {
	return &modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.AccessLists: {Enabled: true},
			},
		},
	}
}

type fakeAuthorizer struct {
	checker *fakeChecker
}

func (f *fakeAuthorizer) Authorize(ctx context.Context) (*authz.Context, error) {
	identity, err := authz.UserFromContext(ctx)
	if err == nil {
		return &authz.Context{
			Identity: identity,
			Checker:  f.checker,
		}, nil
	}

	return &authz.Context{
		Identity: &authz.LocalUser{
			Username: "test-user",
			Identity: tlsca.Identity{},
		},
		Checker: f.checker,
	}, nil
}

type fakeChecker struct {
	services.AccessChecker
	rules []types.Rule
	roles []string
}

func (f *fakeChecker) GuessIfAccessIsPossible(context services.RuleContext, namespace string, kind string, verb string) error {
	for _, r := range f.rules {
		if r.HasResource(kind) && r.HasVerb(verb) {
			return nil
		}
	}
	return trace.AccessDenied("access to %s with verb %s is not allowed", kind, verb)
}

func (f *fakeChecker) CheckAccessToRule(context services.RuleContext, namespace string, kind string, verb string) error {
	for _, r := range f.rules {
		if r.HasResource(kind) && r.HasVerb(verb) {
			return nil
		}
	}
	return trace.AccessDenied("access to %s with verb %s is not allowed", kind, verb)
}

// HasRole checks if the checker includes the role
func (f *fakeChecker) HasRole(target string) bool {
	return slices.Contains(f.roles, target)
}

func assertAccessDenied(t require.TestingT, err error, msg ...any) {
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err), "expected error to be AccessDenied, got %v instead", err)
}

func assertNotFound(t require.TestingT, err error, msg ...any) {
	require.Error(t, err)
	require.True(t, trace.IsNotFound(err), "expected error to be NotFound, got %v instead", err)
}
