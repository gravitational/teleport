package pluginsv1

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/plugins"
	"github.com/gravitational/teleport/integrations/access/common/auth/storage"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
)

func TestMain(m *testing.M) {
	modules.SetInsecureTestMode(true)
	os.Exit(m.Run())
}

func TestGetPluginWithSecrets(t *testing.T) {
	t.Parallel()

	const pluginName = "slack-default"
	ctx := context.Background()
	suite := createSuite(t)
	suite.createPlugin(t, pluginName)

	tt := []struct {
		Name                        string
		Rules                       []types.Rule
		ErrAssertionRead            require.ErrorAssertionFunc
		ErrAssertionReadWithSecrets require.ErrorAssertionFunc
	}{
		{
			Name:                        "deny if user has no rules regarding plugins",
			Rules:                       nil,
			ErrAssertionRead:            assertAccessDenied,
			ErrAssertionReadWithSecrets: assertAccessDenied,
		},
		{
			Name: "deny secrets if user has ReadNoSecrets",
			Rules: []types.Rule{
				{
					Resources: []string{types.KindPlugin},
					Verbs:     []string{types.VerbReadNoSecrets},
				},
			},
			ErrAssertionRead:            require.NoError,
			ErrAssertionReadWithSecrets: assertAccessDenied,
		},
		{
			Name: "allow if user has Read",
			Rules: []types.Rule{
				{
					Resources: []string{types.KindPlugin},
					Verbs:     []string{types.VerbRead},
				},
			},
			ErrAssertionRead:            require.NoError,
			ErrAssertionReadWithSecrets: require.NoError,
		},
	}

	for _, tc := range tt {
		t.Run(tc.Name, func(t *testing.T) {
			suite.setRules(tc.Rules)

			t.Run("without secrets", func(t *testing.T) {
				_, err := suite.svc.GetPlugin(ctx, &pluginspb.GetPluginRequest{Name: pluginName})
				tc.ErrAssertionRead(t, err)
			})
			t.Run("with secrets", func(t *testing.T) {
				_, err := suite.svc.GetPlugin(ctx, &pluginspb.GetPluginRequest{Name: pluginName, WithSecrets: true})
				tc.ErrAssertionReadWithSecrets(t, err)
			})
		})
	}

	t.Run("relay notfound when user has list permission", func(t *testing.T) {
		suite.setRules([]types.Rule{
			{
				Resources: []string{types.KindPlugin},
				Verbs:     []string{types.VerbReadNoSecrets, types.VerbList},
			},
		})
		_, err := suite.svc.GetPlugin(ctx, &pluginspb.GetPluginRequest{Name: "non-existent", WithSecrets: true})
		assertNotFound(t, err)
	})

	t.Run("do not relay notfound when user has no list permission", func(t *testing.T) {
		suite.setRules([]types.Rule{
			{
				Resources: []string{types.KindPlugin},
				Verbs:     []string{},
			},
		})
		_, err := suite.svc.GetPlugin(ctx, &pluginspb.GetPluginRequest{Name: "non-existent", WithSecrets: true})
		assertAccessDenied(t, err)
	})
}

func TestSetPluginCredentials(t *testing.T) {
	t.Parallel()

	const pluginName = "slack-default"
	ctx := context.Background()

	suite := createSuite(t)
	suite.createPlugin(t, pluginName)

	tt := []struct {
		Name         string
		Rules        []types.Rule
		ErrAssertion require.ErrorAssertionFunc
	}{
		{
			Name:         "deny if user has no rules regarding plugins",
			Rules:        nil,
			ErrAssertion: assertAccessDenied,
		},
		{
			Name: "deny secrets if user can update, but can not read secrets",
			Rules: []types.Rule{
				{
					Resources: []string{types.KindPlugin},
					Verbs:     []string{types.VerbReadNoSecrets, types.VerbUpdate},
				},
			},
			ErrAssertion: assertAccessDenied,
		},
		{
			Name: "deny if user can read secrets, but not update",
			Rules: []types.Rule{
				{
					Resources: []string{types.KindPlugin},
					Verbs:     []string{types.VerbRead},
				},
			},
			ErrAssertion: assertAccessDenied,
		},
		{
			Name: "allow if user can read secrets, and update",
			Rules: []types.Rule{
				{
					Resources: []string{types.KindPlugin},
					Verbs:     []string{types.VerbRead, types.VerbUpdate},
				},
			},
			ErrAssertion: require.NoError,
		},
	}

	for _, tc := range tt {
		t.Run(tc.Name, func(t *testing.T) {
			suite.setRules(tc.Rules)

			_, err := suite.svc.SetPluginCredentials(ctx, &pluginspb.SetPluginCredentialsRequest{
				Name: pluginName,
				Credentials: &types.PluginCredentialsV1{
					Credentials: &types.PluginCredentialsV1_Oauth2AccessToken{
						Oauth2AccessToken: &types.PluginOAuth2AccessTokenCredentials{
							AccessToken:  "foo",
							RefreshToken: "bar",
							Expires:      time.Now().Add(1 * time.Minute),
						},
					},
				},
			})
			tc.ErrAssertion(t, err)
		})
	}
}

func TestSearchPluginStaticCredentials(t *testing.T) {
	t.Parallel()

	const pluginName = "okta"

	suite := createSuite(t)

	plugin := &types.PluginV1{
		Metadata: types.Metadata{
			Name: pluginName,
		},
		Spec: types.PluginSpecV1{
			Settings: &types.PluginSpecV1_Okta{
				Okta: &types.PluginOktaSettings{
					OrgUrl: "https://www.okta.com",
				},
			},
		},
		Credentials: &types.PluginCredentialsV1{
			Credentials: &types.PluginCredentialsV1_StaticCredentialsRef{
				StaticCredentialsRef: &types.PluginStaticCredentialsRef{
					Labels: map[string]string{
						"label1": "value1",
					},
				},
			},
		},
	}

	ctx := context.Background()

	err := suite.pluginService.CreatePlugin(ctx, plugin)
	require.NoError(t, err)

	cred := &types.PluginStaticCredentialsV1{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{
				Name: "cred",
				Labels: map[string]string{
					"label1": "value1",
				},
			},
		},
		Spec: &types.PluginStaticCredentialsSpecV1{
			Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
				APIToken: "api-token",
			},
		},
	}

	require.NoError(t, suite.pluginStaticCredentialsService.CreatePluginStaticCredentials(ctx, cred))

	tt := []struct {
		name         string
		identity     authz.IdentityGetter
		roles        []string
		labels       map[string]string
		expected     []*types.PluginStaticCredentialsV1
		errAssertion require.ErrorAssertionFunc
	}{
		{
			name:     "non admin or proxy",
			identity: authtest.TestUser("someuser").I,
			roles:    []string{},
			expected: nil,
			errAssertion: func(tt require.TestingT, err error, i ...any) {
				require.ErrorIs(t, err, trace.AccessDenied("access denied"))
			},
		},
		{
			name:     "admin gets arbitrary cred",
			identity: authtest.TestBuiltin(types.RoleAdmin).I,
			roles:    []string{string(types.RoleAdmin)},
			labels: map[string]string{
				"label1": "value1",
			},
			expected: []*types.PluginStaticCredentialsV1{
				cred,
			},
			errAssertion: require.NoError,
		},
		{
			name:     "proxy gets access denied asking for arbitrary cred",
			identity: authtest.TestBuiltin(types.RoleProxy).I,
			roles:    []string{string(types.RoleProxy)},
			labels: map[string]string{
				"label1": "value1",
			},
			expected: nil,
			errAssertion: func(tt require.TestingT, err error, i ...any) {
				require.ErrorIs(t, err, trace.AccessDenied("access denied"))
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			ctx := authz.ContextWithUser(ctx, tc.identity)
			suite.setRoles(tc.roles)

			resp, err := suite.svc.SearchPluginStaticCredentials(ctx, &pluginspb.SearchPluginStaticCredentialsRequest{Labels: tc.labels})
			tc.errAssertion(t, err)

			if tc.expected == nil {
				require.Nil(t, resp)
			} else if tc.expected != nil {
				require.NotNil(t, resp)
				require.Empty(t, cmp.Diff(tc.expected, resp.Credentials, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
			}
		})
	}
}

func TestGetAvailablePluginTypes(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	suite := createSuite(t)
	suite.setRules([]types.Rule{
		{Resources: []string{types.KindPlugin}, Verbs: services.RW()},
	})

	// create a noop authorizer that should show up when getting plugin types.
	slackAuthorizer := &mockAuthorizer{
		exchange: func(authCode string, redirectURI string) (*storage.Credentials, error) {
			return nil, trace.NotImplemented("not implemented")
		},
	}

	suite.pluginAuthorizers.Add(types.PluginTypeSlack, &plugins.Authorizer{Authorizer: slackAuthorizer, ClientID: "123456"})

	resp, err := suite.svc.GetAvailablePluginTypes(ctx, &pluginspb.GetAvailablePluginTypesRequest{})
	require.NoError(t, err)

	require.ElementsMatch(t, []*pluginspb.PluginType{
		{
			Type:          types.PluginTypeSlack,
			OauthClientId: "123456",
		},
		{
			Type: types.PluginTypeOkta,
		},
		{
			Type: types.PluginTypeJamf,
		},
		{
			Type: types.PluginTypeJira,
		},
		{
			Type: types.PluginTypeServiceNow,
		},
		{
			Type: types.PluginTypeOpsgenie,
		},
		{
			Type: types.PluginTypePagerDuty,
		},
		{
			Type: types.PluginTypeMattermost,
		},
		{
			Type: types.PluginTypeDiscord,
		},
		{
			Type: types.PluginTypeGitlab,
		},
		{
			Type: types.PluginTypeEntraID,
		},
		{
			Type: types.PluginTypeDatadog,
		},
		{
			Type: types.PluginTypeAWSIdentityCenter,
		},
		{
			Type: types.PluginTypeMSTeams,
		},
		{
			Type: types.PluginTypeEmail,
		},
		{
			Type: types.PluginTypeGithub,
		},
		{
			Type: types.PluginTypeSCIM,
		},
	}, resp.PluginTypes)
}
