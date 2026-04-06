package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/externalauditstorage"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
)

func Test_getBlockedPlugins(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want []types.PluginType
	}{
		{
			name: "no env",
			env:  "",
			want: nil,
		},
		{
			name: "single plugin",
			env:  types.PluginTypeAWSIdentityCenter,
			want: []types.PluginType{types.PluginTypeAWSIdentityCenter},
		},
		{
			name: "multiple plugins",
			env:  strings.Join([]string{types.PluginTypeAWSIdentityCenter, types.PluginTypeEntraID}, ","),
			want: []types.PluginType{types.PluginTypeAWSIdentityCenter, types.PluginTypeEntraID},
		},
		{
			name: "multiple plugins with whitespaces",
			env:  strings.Join([]string{types.PluginTypeAWSIdentityCenter, types.PluginTypeEntraID}, "  ,  "),
			want: []types.PluginType{types.PluginTypeAWSIdentityCenter, types.PluginTypeEntraID},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(envVarNameDisabledPlugins, tt.env)
			require.ElementsMatch(t, tt.want, getDisabledPlugins())
		})
	}
}

func TestNewExternalAuditStorageConfigurator(t *testing.T) {
	ctx := t.Context()

	tests := []struct {
		name           string
		modules        modulestest.Modules
		backendSetupFn func(t *testing.T, b backend.Backend)
		wantNil        bool
		isUsed         bool
	}{
		{
			name: "cloud features not enabled",
			modules: modulestest.Modules{
				TestFeatures: modules.Features{
					Cloud: false,
				},
			},
			wantNil: true,
		},
		{
			name: "cloud features enabled with valid backend but not licensed for external audit storage",
			modules: modulestest.Modules{
				TestFeatures: modules.Features{
					Cloud: true,
					Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
						entitlements.ExternalAuditStorage: {Enabled: false},
					},
				},
			},
			wantNil: false,
		},
		{
			name: "cloud features enabled with valid backend but no external audit storage configured",
			modules: modulestest.Modules{
				TestFeatures: modules.Features{
					Cloud: true,
					Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
						entitlements.ExternalAuditStorage: {Enabled: true},
					},
				},
			},
			wantNil: false,
		},
		{
			name: "cloud features enabled and external audit storage configured",
			modules: modulestest.Modules{
				TestFeatures: modules.Features{
					Cloud: true,
					Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
						entitlements.ExternalAuditStorage: {Enabled: true},
					},
				},
			},
			backendSetupFn: createEASSetup,
			wantNil:        false,
			isUsed:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modulestest.SetTestModules(t, tt.modules)

			plugin := newPluginPack(t, &tt.modules)

			if tt.backendSetupFn != nil {
				tt.backendSetupFn(t, plugin.authServer.GetBackend())
			}

			configurator, err := plugin.newExternalAuditStorageConfigurator(ctx)

			require.NoError(t, err)

			if tt.wantNil {
				require.Nil(t, configurator)
			} else {
				require.NotNil(t, configurator)
				require.Equal(t, tt.isUsed, configurator.IsUsed())
			}
		})
	}
}

func createEASSetup(t *testing.T, b backend.Backend) {
	t.Helper()
	ctx := t.Context()
	easSvc := local.NewExternalAuditStorageService(b)
	integrationSvc, err := local.NewIntegrationsService(b)
	require.NoError(t, err)

	integration, err := types.NewIntegrationAWSOIDC(
		types.Metadata{Name: "test-integration"},
		&types.AWSOIDCIntegrationSpecV1{
			RoleARN: "arn:aws:iam::123456789012:role/test-role",
		},
	)
	require.NoError(t, err)
	_, err = integrationSvc.CreateIntegration(ctx, integration)
	require.NoError(t, err)

	// Create an ExternalAuditStorage resource that references the integration
	eas, err := externalauditstorage.NewDraftExternalAuditStorage(
		header.Metadata{},
		externalauditstorage.ExternalAuditStorageSpec{
			IntegrationName:        integration.GetName(),
			PolicyName:             "test-policy",
			Region:                 "us-west-2",
			SessionRecordingsURI:   "s3://test-bucket/sessions",
			AuditEventsLongTermURI: "s3://test-bucket/events",
			AthenaResultsURI:       "s3://test-bucket/results",
			AthenaWorkgroup:        "test_workgroup",
			GlueDatabase:           "test_database",
			GlueTable:              "test_table",
		},
	)
	require.NoError(t, err)
	_, err = easSvc.CreateDraftExternalAuditStorage(ctx, eas)
	require.NoError(t, err)

	err = easSvc.PromoteToClusterExternalAuditStorage(ctx)
	require.NoError(t, err)
}

func newPluginPack(t *testing.T, m *modulestest.Modules) *Plugin {
	t.Helper()
	ctx := t.Context()
	clock := clockwork.NewFakeClockAt(time.Now())

	b, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	t.Cleanup(func() { b.Close() })

	clusterName, err := services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: "me.localhost",
	})
	require.NoError(t, err)

	authConfig := &auth.InitConfig{
		Backend:                b,
		SkipPeriodicOperations: true,
		VersionStorage:         authtest.NewFakeTeleportVersion(),
		HostUUID:               uuid.NewString(),
		ClusterName:            clusterName,
		Modules:                m,
	}
	a, err := auth.NewServer(authConfig)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, a.Close()) })

	return &Plugin{
		Config: Config{
			Modules: m,
		},
		authServer: &auth.GRPCServer{
			APIConfig: auth.APIConfig{
				AuthServer: a,
			},
		},
	}
}
