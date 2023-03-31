package pluginsv1

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/plugins"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
)

type suite struct {
	authorizer        *fakeAuthorizer
	backendService    services.Plugins
	pluginAuthorizers *plugins.AuthorizerSet
	svc               *Service
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

	err := s.backendService.CreatePlugin(context.Background(), plugin)
	require.NoError(t, err)
}

func (s *suite) setRules(rules []types.Rule) {
	s.authorizer.checker.rules = rules
}

func createSuite(t *testing.T) *suite {
	mem, err := memory.New(memory.Config{
		Clock: clockwork.NewFakeClock(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, mem.Close()) })

	authorizer := &fakeAuthorizer{checker: &fakeChecker{}}
	backendService := local.NewPluginsService(mem)
	pluginAuthorizers := plugins.NewAuthorizerSet()

	return &suite{
		authorizer:        authorizer,
		backendService:    backendService,
		pluginAuthorizers: pluginAuthorizers,
		svc: &Service{
			authorizer:        authorizer,
			backendService:    backendService,
			pluginAuthorizers: pluginAuthorizers,
		},
	}
}

type fakeAuthorizer struct {
	checker *fakeChecker
}

func (f *fakeAuthorizer) Authorize(ctx context.Context) (*authz.Context, error) {
	return &authz.Context{
		Checker: f.checker,
	}, nil
}

type fakeChecker struct {
	services.AccessChecker
	rules []types.Rule
}

func (f *fakeChecker) CheckAccessToRule(context services.RuleContext, namespace string, kind string, verb string, silent bool) error {
	for _, r := range f.rules {
		if r.HasResource(kind) && r.HasVerb(verb) {
			return nil
		}
	}
	return trace.AccessDenied("access to %s with verb %s is not allowed", kind, verb)
}

func assertAccessDenied(t require.TestingT, err error, msg ...interface{}) {
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err), "expected error to be AccessDenied, got %v instead", err)
}
