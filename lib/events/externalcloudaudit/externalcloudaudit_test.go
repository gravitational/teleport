package externalcloudaudit

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types/externalcloudaudit"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
)

func TestConfiguratorIsUsed(t *testing.T) {
	draftConfig, err := externalcloudaudit.NewDraftExternalCloudAudit(header.Metadata{},
		externalcloudaudit.ExternalCloudAuditSpec{
			IntegrationName:        "aws-integration-1",
			SessionsRecordingsURI:  "s3://bucket/sess_rec",
			AthenaWorkgroup:        "primary",
			GlueDatabase:           "teleport_db",
			GlueTable:              "teleport_table",
			AuditEventsLongTermURI: "s3://bucket/events",
			AthenaResultsURI:       "s3://bucket/results",
		})
	require.NoError(t, err)
	tests := []struct {
		name              string
		modules           *modules.TestModules
		resourceServiceFn func(t *testing.T, s services.ExternalCloudAudits)
		wantIsUsed        bool
	}{
		{
			name: "not cloud - cloud external audit disabled",
			modules: &modules.TestModules{
				TestFeatures: modules.Features{
					Cloud: false,
				},
			},
			wantIsUsed: false,
		},
		{
			name: "cloud team - cloud external audit disabled",
			modules: &modules.TestModules{
				TestFeatures: modules.Features{
					Cloud:               true,
					IsUsageBasedBilling: true,
				},
			},
			wantIsUsed: false,
		},
		{
			name: "cloud enterprise without config - cloud external audit disabled",
			modules: &modules.TestModules{
				TestFeatures: modules.Features{
					Cloud:               true,
					IsUsageBasedBilling: false,
				},
			},
			wantIsUsed: false,
		},
		{
			name: "cloud enterprise with only draft - cloud external audit disabled",
			modules: &modules.TestModules{
				TestFeatures: modules.Features{
					Cloud:               true,
					IsUsageBasedBilling: false,
				},
			},
			// just create draft, external cloud audit should be disabled, it's working
			// only on cluster external cloud resource.
			resourceServiceFn: func(t *testing.T, s services.ExternalCloudAudits) {
				_, err := s.UpsertDraftExternalCloudAudit(context.Background(), draftConfig)
				require.NoError(t, err)
			},
			wantIsUsed: false,
		},
		{
			name: "cloud enterprise with cluster config - cloud external audit enabled",
			modules: &modules.TestModules{
				TestFeatures: modules.Features{
					Cloud:               true,
					IsUsageBasedBilling: false,
				},
			},
			// create draft and promote it to cluster.
			resourceServiceFn: func(t *testing.T, s services.ExternalCloudAudits) {
				_, err := s.UpsertDraftExternalCloudAudit(context.Background(), draftConfig)
				require.NoError(t, err)
				err = s.PromoteToClusterExternalCloudAudit(context.Background())
				require.NoError(t, err)
			},
			wantIsUsed: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mem, err := memory.New(memory.Config{})
			require.NoError(t, err)

			svc := local.NewExternalCloudAuditService(mem)
			if tt.resourceServiceFn != nil {
				tt.resourceServiceFn(t, svc)
			}

			modules.SetTestModules(t, tt.modules)

			c, err := NewConfigurator(context.Background(), mem)
			require.NoError(t, err)
			if got := c.IsUsed(); got != tt.wantIsUsed {
				t.Errorf("Configurator.IsUsed() = %v, want %v", got, tt.wantIsUsed)
			}
		})
	}
}
