// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package externalcloudaudit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
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
			PolicyName:             "ecaPolicy",
			Region:                 "us-west-2",
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

func TestCredentialsCache(t *testing.T) {
	cc, err := newCredentialsCache("test")
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go cc.run(ctx)
	// TODO(nklaassen): rework to use from sdkv1/v2

	t.Run("retrieve fn not initialized yet, expect err", func(t *testing.T) {
		// This test case covers scenario when auth is not yet initialized.
		_, err := cc.getCredentialsFromCache(ctx)
		require.ErrorContains(t, err, "cache not yet initialized")
	})

	mock := mockCredentialsRetrieve{
		wantResp: func() (aws.Credentials, error) {
			return aws.Credentials{}, errors.New("error from credential retrieve")
		},
	}
	t.Run("set retrieve fn to return error", func(t *testing.T) {
		cc.SetRetrieveCredentialsFn(mock.Retrieve)
		// This test case covers scenario when cache is initialized
		// however retrieve fn returns error.
		require.Eventually(t, func() bool {
			_, err := cc.getCredentialsFromCache(ctx)
			return assert.ErrorContains(t, err, "error from credential retrieve")
		}, 5*time.Second, 100*time.Millisecond)
	})
}

type mockCredentialsRetrieve struct {
	wantResp func() (aws.Credentials, error)
}

func (m *mockCredentialsRetrieve) Retrieve(ctx context.Context, integration string) (aws.Credentials, error) {
	return m.wantResp()
}
