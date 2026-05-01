package awsic

import (
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	icsdk "github.com/gravitational/teleport/e/lib/aws/identitycenter/sdk"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
	"github.com/gravitational/teleport/e/tests/common"
	"github.com/gravitational/teleport/e/tests/common/idp"
)

func TestInstall(t *testing.T) {
	const pluginName = "aws-identity-center"

	ctx := t.Context()
	setupMockAWSICEnvironment(t, icsdk.NewClientMock(nil), scimsdk.NewSCIMClientMock())

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.SAMLConnector),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithRole(t, "aws-ic-access", common.WithAccountAssignment(types.Allow, "*", "*")),
		common.WithUser(t, "alice", "editor"),
	)

	auth := sut.Teleport.Process.GetAuthServer()
	mustSetupOIDCIntegration(t, auth)

	tctl := sut.GetTCTL(t)

	commonArgs := []string{
		"plugins", "install", "awsic",
		"--access-list-default-owner", "alice",
	}

	testCases := []struct {
		name            string
		args            []string
		errorAssertion  require.ErrorAssertionFunc
		pluginAssertion func(*testing.T, *types.PluginV1)
	}{
		{
			name: "system credentials",
			args: []string{
				"--use-system-credentials",
				"--assume-role-arn", "arn:aws:iam::123456789012:role/idc-integration",
				"--scim-url", "https://scim.us-east-1.amazonaws.com/311ce970-dce2-4736-8a22-773e5f8586bf/scim/v2",
				"--scim-token", "hey-let-me-in",
				"--instance-arn", "arn:aws:sso:::instance/ssoins-123456789aaaa",
				"--instance-region", "us-east-1",
				"--roles-sync-mode", "NONE",
			},
			errorAssertion: require.NoError,
			pluginAssertion: func(t *testing.T, plugin *types.PluginV1) {
				settings := plugin.Spec.GetAwsIc()
				require.NotNil(t, settings)

				systemCreds := settings.Credentials.GetSystem()
				require.NotNil(t, systemCreds)
			},
		},
		{
			name: "OIDC credentials",
			args: []string{
				"--no-use-system-credentials",
				"--oidc-integration", oidcIntegrationName,
				"--scim-url", "https://scim.us-east-1.amazonaws.com/311ce970-dce2-4736-8a22-773e5f8586bf/scim/v2",
				"--scim-token", "hey-let-me-in",
				"--instance-arn", "arn:aws:sso:::instance/ssoins-123456789aaaa",
				"--instance-region", "us-east-1",
				"--roles-sync-mode", "NONE",
			},
			errorAssertion: require.NoError,
			pluginAssertion: func(t *testing.T, plugin *types.PluginV1) {
				settings := plugin.Spec.GetAwsIc()
				require.NotNil(t, settings)

				oidcCreds := settings.Credentials.GetOidc()
				require.NotNil(t, oidcCreds)
			},
		},
		{
			name: "OIDC credentials with no such integration",
			args: []string{
				"--no-use-system-credentials",
				"--oidc-integration", "no-such-integration",
				"--scim-url", "https://scim.us-east-1.amazonaws.com/311ce970-dce2-4736-8a22-773e5f8586bf/scim/v2",
				"--scim-token", "hey-let-me-in",
				"--instance-arn", "arn:aws:sso:::instance/ssoins-123456789aaaa",
				"--instance-region", "us-east-1",
				"--roles-sync-mode", "NONE",
			},
			errorAssertion: require.Error,
		},
	}

	// TODO(tcsc): maybe replace with cleanup used during real install
	cleanupPlugin := func(t *testing.T) func() {
		return func() {
			auth.DeletePlugin(ctx, pluginName)
			auth.DeletePluginStaticCredentials(ctx, pluginName)
			require.EventuallyWithT(t,
				func(t *assert.CollectT) {
					var notFound *trace.NotFoundError

					_, err := auth.GetPlugin(ctx, pluginName, false)
					require.ErrorAs(t, err, &notFound)

					_, err = auth.GetPluginStaticCredentials(ctx, pluginName)
					require.ErrorAs(t, err, &notFound)
				},
				3*time.Second, 100*time.Millisecond)
		}
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Cleanup(cleanupPlugin(t))

			err := tctl.Run(ctx, append(commonArgs, testCase.args...)...)
			testCase.errorAssertion(t, err)

			if testCase.pluginAssertion != nil {
				var wrappedPlugin types.Plugin
				require.EventuallyWithT(t, func(t *assert.CollectT) {
					var err error
					wrappedPlugin, err = auth.GetPlugin(ctx, pluginName, false)
					require.NoError(t, err)
				}, 3*time.Second, 100*time.Millisecond)

				plugin, ok := wrappedPlugin.(*types.PluginV1)
				require.True(t, ok, "Unexpected plugin type: %T", wrappedPlugin)
				testCase.pluginAssertion(t, plugin)
			}
		})
	}
}
