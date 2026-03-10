package pluginsv1

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	pluginsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/services"
)

func TestPluginUpdate(t *testing.T) {
	t.Parallel()
	suite := createSuite(t)
	ctx := context.Background()
	plugin := &types.PluginV1{
		Metadata: types.Metadata{Name: "okta"},
		Spec: types.PluginSpecV1{
			Settings: &types.PluginSpecV1_Okta{
				Okta: &types.PluginOktaSettings{
					OrgUrl: "https://okta.com",
					SyncSettings: &types.PluginOktaSyncSettings{
						SyncUsers:      true,
						SsoConnectorId: "conn_id",
					},
				},
			},
		},
	}

	suite.setRules([]types.Rule{{Resources: []string{types.KindPlugin}, Verbs: services.RO()}})
	_, err := suite.svc.UpdatePlugin(ctx, &pluginsv1.UpdatePluginRequest{
		Plugin: plugin,
	})
	require.True(t, trace.IsAccessDenied(err))

	suite.setRules([]types.Rule{{Resources: []string{types.KindPlugin}, Verbs: services.RW()}})

	_, err = suite.svc.UpdatePlugin(ctx, &pluginsv1.UpdatePluginRequest{
		Plugin: plugin,
	})
	require.True(t, trace.IsNotFound(err))

	_, err = suite.svc.CreatePlugin(ctx, &pluginsv1.CreatePluginRequest{
		Plugin: plugin,
		StaticCredentials: &types.PluginStaticCredentialsV1{
			ResourceHeader: types.ResourceHeader{
				Metadata: types.Metadata{
					Name: "okta",
				},
			},
			Spec: &types.PluginStaticCredentialsSpecV1{
				Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
					APIToken: "api_token",
				},
			},
		},
	})
	require.NoError(t, err)

	p, err := suite.svc.GetPlugin(ctx, &pluginsv1.GetPluginRequest{
		Name:        "okta",
		WithSecrets: false,
	})
	require.NoError(t, err)

	p.Spec.GetOkta().SsoConnectorId = "new_sso_id"

	got, err := suite.svc.UpdatePlugin(ctx, &pluginsv1.UpdatePluginRequest{
		Plugin: p,
	})
	require.NoError(t, err)

	require.Empty(t, cmp.Diff(p, got, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

	// Test that disabled plugins can't be updated.
	suite.svc.disabledPlugins = []types.PluginType{plugin.GetType()}
	got, err = suite.svc.UpdatePlugin(ctx, &pluginsv1.UpdatePluginRequest{
		Plugin: got,
	})
	require.Error(t, err)
	require.Nil(t, got)
	require.True(t, trace.IsBadParameter(err))

	t.Run("nil plugin should not panic", func(t *testing.T) {
		_, err = suite.svc.UpdatePlugin(ctx, &pluginsv1.UpdatePluginRequest{
			Plugin: nil,
		})
		require.Error(t, err)
	})
}

// TestPluginUpdateHandler tests that the supplied plugin resource gets correctly
// updated and validated being committed to the back end data store
func TestPluginUpdateHandler(t *testing.T) {
	t.Parallel()
	now := time.Now()

	newValidOktaStatus := func() *types.PluginV1 {
		return &types.PluginV1{
			Spec: types.PluginSpecV1{
				Settings: &types.PluginSpecV1_Okta{Okta: &types.PluginOktaSettings{}},
			},
			Status: types.PluginStatusV1{
				Code:         types.PluginStatusCode_RUNNING,
				LastSyncTime: now,
				Details: &types.PluginStatusV1_Okta{
					Okta: &types.PluginOktaStatusV1{
						SsoDetails: &types.PluginOktaStatusDetailsSSO{
							Enabled: true,
							AppId:   "some-app-id",
							AppName: "A Valid App Name",
						},
						AppGroupSyncDetails: &types.PluginOktaStatusDetailsAppGroupSync{
							Enabled:         true,
							LastSuccessful:  &now,
							NumAppsSynced:   42,
							NumGroupsSynced: 84,
						},
					},
				},
			},
		}
	}

	testCases := []struct {
		name                 string
		makePlugin           func() *types.PluginV1
		mutateExistingPlugin func(*types.PluginV1)
		mutateNewPlugin      func(*types.PluginV1)
		expectedResult       require.ErrorAssertionFunc
		mutateExpectedPlugin func(*types.PluginV1)
	}{
		{
			name:       "Okta status is preserved",
			makePlugin: newValidOktaStatus,
			mutateNewPlugin: func(p *types.PluginV1) {
				p.Status = types.PluginStatusV1{}
			},
			expectedResult: require.NoError,
		},
		{
			name:       "Okta rejects status changes",
			makePlugin: newValidOktaStatus,
			mutateNewPlugin: func(p *types.PluginV1) {
				details := p.GetStatus().GetOkta()
				details.SsoDetails = nil
				details.AppGroupSyncDetails = nil
				details.UsersSyncDetails = nil
			},
			expectedResult: require.NoError,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			existingPlugin := test.makePlugin()
			if test.mutateExistingPlugin != nil {
				test.mutateExistingPlugin(existingPlugin)
			}

			newPlugin := test.makePlugin()
			if test.mutateNewPlugin != nil {
				test.mutateNewPlugin(newPlugin)
			}

			handler, ok := defaultPluginHandlers[newPlugin.GetType()]
			require.True(t, ok, "missing validator for plugin type %q", newPlugin.GetType())

			err := handler.updatePlugin(newPlugin, existingPlugin)
			test.expectedResult(t, err)

			expectedPlugin := test.makePlugin()
			if test.mutateExpectedPlugin != nil {
				test.mutateExpectedPlugin(expectedPlugin)
			}
			require.Equal(t, expectedPlugin, newPlugin)
		})
	}
}

func TestPluginStatusTrimmed(t *testing.T) {
	s := createSuite(t)
	s.setRules([]types.Rule{
		{Resources: []string{types.KindPlugin}, Verbs: services.RW()},
	})
	ctx := t.Context()

	plugin := utils.CloneProtoMsg(entraPlugin(t))
	require.NoError(t, s.pluginService.CreatePlugin(context.Background(), plugin))

	testCases := []struct {
		name         string
		maxSize      int
		in           *types.PluginStatusV1
		expectNoTrim bool
	}{
		{
			name: "exceeds max size",
			in: &types.PluginStatusV1{
				LastSyncTime: time.Now(),
				ErrorMessage: "Failed to sync, access denied.",
				LastRawError: strings.Repeat("A", maxDynamoDBItemSize),
			},
		},
		{
			name: "does not exceed max size",
			in: &types.PluginStatusV1{
				ErrorMessage: "Failed to sync, access denied.",
				LastRawError: strings.Repeat("A", 5500),
			},
			expectNoTrim: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pluginBefore, err := s.svc.GetPlugin(ctx, &pluginsv1.GetPluginRequest{
				Name:        types.PluginTypeEntraID,
				WithSecrets: false,
			})
			require.NoError(t, err)
			pluginBefore.SetStatus(tc.in)

			_, err = s.svc.SetPluginStatus(ctx, &pluginsv1.SetPluginStatusRequest{
				Name:   types.PluginTypeEntraID,
				Status: tc.in,
			})
			require.NoError(t, err)

			pluginAfter, err := s.svc.GetPlugin(ctx, &pluginsv1.GetPluginRequest{
				Name:        types.PluginTypeEntraID,
				WithSecrets: false,
			})
			require.NoError(t, err)

			if tc.expectNoTrim {
				require.Equal(t, pluginBefore.Size(), pluginAfter.Size())
			} else {
				require.LessOrEqual(t, pluginAfter.Size(), pluginBefore.Size())
				require.LessOrEqual(t, pluginAfter.Size(), maxDynamoDBItemSize)
			}
		})
	}
}

func entraPlugin(t *testing.T) *types.PluginV1 {
	t.Helper()
	return &types.PluginV1{
		Metadata: types.Metadata{
			Name: types.PluginTypeEntraID,
		},
		Spec: types.PluginSpecV1{
			Settings: &types.PluginSpecV1_EntraId{
				EntraId: &types.PluginEntraIDSettings{
					SyncSettings: &types.PluginEntraIDSyncSettings{
						DefaultOwners:  []string{"testuser"},
						TenantId:       "testid",
						EntraAppId:     "testid",
						SsoConnectorId: "testconnector",
						GroupFilters: []*types.PluginSyncFilter{
							{Include: &types.PluginSyncFilter_NameRegex{NameRegex: "^[)$"}},
						},
					},
				},
			},
		},
	}
}
