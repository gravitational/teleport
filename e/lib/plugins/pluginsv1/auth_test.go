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
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/services"
)

func TestMain(m *testing.M) {
	modules.SetInsecureTestMode(true)
	modules.SetModules(&modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.AccessLists: {Enabled: true},
			},
		},
	})
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
				_, err := suite.svc.GetPlugin(ctx, pluginspb.GetPluginRequest_builder{Name: pluginName}.Build())
				tc.ErrAssertionRead(t, err)
			})
			t.Run("with secrets", func(t *testing.T) {
				_, err := suite.svc.GetPlugin(ctx, pluginspb.GetPluginRequest_builder{Name: pluginName, WithSecrets: true}.Build())
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
		_, err := suite.svc.GetPlugin(ctx, pluginspb.GetPluginRequest_builder{Name: "non-existent", WithSecrets: false}.Build())
		assertNotFound(t, err)
	})

	t.Run("relay access denied when plugin is not found and user misses list permission", func(t *testing.T) {
		suite.setRules([]types.Rule{
			{
				Resources: []string{types.KindPlugin},
				Verbs:     []string{types.VerbRead},
			},
		})
		_, err := suite.svc.GetPlugin(ctx, pluginspb.GetPluginRequest_builder{Name: "non-existent", WithSecrets: true}.Build())
		assertAccessDenied(t, err)
	})

	t.Run("do not relay notfound when user has no list permission", func(t *testing.T) {
		suite.setRules([]types.Rule{
			{
				Resources: []string{types.KindPlugin},
				Verbs:     []string{},
			},
		})
		_, err := suite.svc.GetPlugin(ctx, pluginspb.GetPluginRequest_builder{Name: "non-existent", WithSecrets: true}.Build())
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

			_, err := suite.svc.SetPluginCredentials(ctx, pluginspb.SetPluginCredentialsRequest_builder{
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
			}.Build())
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

			resp, err := suite.svc.SearchPluginStaticCredentials(ctx, pluginspb.SearchPluginStaticCredentialsRequest_builder{Labels: tc.labels}.Build())
			tc.errAssertion(t, err)

			if tc.expected == nil {
				require.Nil(t, resp)
			} else if tc.expected != nil {
				require.NotNil(t, resp)
				require.Empty(t, cmp.Diff(tc.expected, resp.GetCredentials(), cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
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

	resp, err := suite.svc.GetAvailablePluginTypes(ctx, &pluginspb.GetAvailablePluginTypesRequest{})
	require.NoError(t, err)

	require.ElementsMatch(t, []*pluginspb.PluginType{
		pluginspb.PluginType_builder{
			Type:          types.PluginTypeSlack,
			OauthClientId: "123456",
		}.Build(),
		pluginspb.PluginType_builder{
			Type: types.PluginTypeOkta,
		}.Build(),
		pluginspb.PluginType_builder{
			Type: types.PluginTypeJamf,
		}.Build(),
		pluginspb.PluginType_builder{
			Type: types.PluginTypeIntune,
		}.Build(),
		pluginspb.PluginType_builder{
			Type: types.PluginTypeJira,
		}.Build(),
		pluginspb.PluginType_builder{
			Type: types.PluginTypeServiceNow,
		}.Build(),
		pluginspb.PluginType_builder{
			Type: types.PluginTypeOpsgenie,
		}.Build(),
		pluginspb.PluginType_builder{
			Type: types.PluginTypePagerDuty,
		}.Build(),
		pluginspb.PluginType_builder{
			Type: types.PluginTypeMattermost,
		}.Build(),
		pluginspb.PluginType_builder{
			Type: types.PluginTypeDiscord,
		}.Build(),
		pluginspb.PluginType_builder{
			Type: types.PluginTypeGitlab,
		}.Build(),
		pluginspb.PluginType_builder{
			Type: types.PluginTypeEntraID,
		}.Build(),
		pluginspb.PluginType_builder{
			Type: types.PluginTypeDatadog,
		}.Build(),
		pluginspb.PluginType_builder{
			Type: types.PluginTypeAWSIdentityCenter,
		}.Build(),
		pluginspb.PluginType_builder{
			Type: types.PluginTypeMSTeams,
		}.Build(),
		pluginspb.PluginType_builder{
			Type: types.PluginTypeEmail,
		}.Build(),
		pluginspb.PluginType_builder{
			Type: types.PluginTypeGithub,
		}.Build(),
		pluginspb.PluginType_builder{
			Type: types.PluginTypeSCIM,
		}.Build(),
	}, resp.GetPluginTypes())
}
