/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package web

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	discoveryconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryconfig/v1"
	integrationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/lib/services"
	libui "github.com/gravitational/teleport/lib/ui"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/web/ui"
)

func TestIntegrationsCreateWithAudience(t *testing.T) {
	t.Parallel()
	wPack := newWebPack(t, 1 /* proxies */)
	proxy := wPack.proxies[0]
	authPack := proxy.authPack(t, "user", []types.Role{services.NewPresetEditorRole()})
	ctx := context.Background()

	const integrationName = "test-integration"
	cases := []struct {
		name     string
		audience string
	}{
		{
			name:     "without audiences",
			audience: types.IntegrationAWSOIDCAudienceUnspecified,
		},
		{
			name:     "with audiences",
			audience: types.IntegrationAWSOIDCAudienceAWSIdentityCenter,
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			createData := ui.Integration{
				Name:    integrationName,
				SubKind: "aws-oidc",
				AWSOIDC: &ui.IntegrationAWSOIDCSpec{
					RoleARN:  "arn:aws:iam::026090554232:role/testrole",
					Audience: test.audience,
				},
			}
			createEndpoint := authPack.clt.Endpoint("webapi", "sites", wPack.server.ClusterName(), "integrations")
			createResp, err := authPack.clt.PostJSON(ctx, createEndpoint, createData)
			require.NoError(t, err)
			require.Equal(t, 200, createResp.Code())

			// check origin label stored in backend
			intgrationResource, err := wPack.server.Auth().GetIntegration(ctx, integrationName)
			require.NoError(t, err)
			require.Equal(t, test.audience, intgrationResource.GetAWSOIDCIntegrationSpec().Audience)

			// check origin label returned in the web api
			getEndpoint := authPack.clt.Endpoint("webapi", "sites", wPack.server.ClusterName(), "integrations", integrationName)
			getResp, err := authPack.clt.Get(ctx, getEndpoint, nil)
			require.NoError(t, err)
			require.Equal(t, 200, getResp.Code())

			var resp ui.Integration
			err = json.Unmarshal(getResp.Bytes(), &resp)
			require.NoError(t, err)
			require.Equal(t, createData, resp)

			err = wPack.server.Auth().DeleteIntegration(ctx, integrationName)
			require.NoError(t, err)
		})
	}
}

func TestCollectAWSOIDCAutoDiscoverStats(t *testing.T) {
	ctx := context.Background()
	logger := utils.NewSlogLoggerForTests()

	integrationName := "my-integration"
	integration, err := types.NewIntegrationAWSOIDC(
		types.Metadata{Name: integrationName},
		&types.AWSOIDCIntegrationSpecV1{
			RoleARN: "arn:role",
		},
	)
	require.NoError(t, err)

	deployedServiceCommand := buildCommandDeployedDatabaseService(t, true, types.Labels{"vpc": []string{"vpc1", "vpc2"}})
	deployedDatabaseServicesClient := &mockDeployedDatabaseServices{
		integration: "my-integration",
		servicesPerRegion: map[string][]*integrationv1.DeployedDatabaseService{
			"us-west-2": dummyDeployedDatabaseServices(1, deployedServiceCommand),
		},
	}

	t.Run("without discovery configs, returns just the integration", func(t *testing.T) {
		clt := &mockRelevantAWSRegionsClient{
			databaseServices: &proto.ListResourcesResponse{
				Resources: []*proto.PaginatedResource{},
			},
			databases:        make([]types.Database, 0),
			discoveryConfigs: make([]*discoveryconfig.DiscoveryConfig, 0),
		}

		gotSummary, err := collectAWSOIDCAutoDiscoverStats(ctx, logger, integration, clt, clt, deployedDatabaseServicesClient)
		require.NoError(t, err)
		expectedSummary := &ui.IntegrationWithSummary{
			Integration: &ui.Integration{
				Name:    integrationName,
				SubKind: "aws-oidc",
				AWSOIDC: &ui.IntegrationAWSOIDCSpec{RoleARN: "arn:role"},
			},
		}
		require.Equal(t, expectedSummary, gotSummary)
	})

	t.Run("collects multiple discovery configs", func(t *testing.T) {
		syncTime := time.Now()
		dcForEC2 := &discoveryconfig.DiscoveryConfig{
			Spec: discoveryconfig.Spec{AWS: []types.AWSMatcher{{
				Integration: integrationName,
				Types:       []string{"ec2"},
				Regions:     []string{"us-east-1"},
			}}},
			Status: discoveryconfig.Status{
				LastSyncTime:        syncTime,
				DiscoveredResources: 2,
				IntegrationDiscoveredResources: map[string]*discoveryconfigv1.IntegrationDiscoveredSummary{
					integrationName: {
						AwsEc2: &discoveryconfigv1.ResourcesDiscoveredSummary{Found: 2, Enrolled: 1, Failed: 1},
					},
				},
			},
		}
		dcForRDS := &discoveryconfig.DiscoveryConfig{
			Spec: discoveryconfig.Spec{AWS: []types.AWSMatcher{{
				Integration: integrationName,
				Types:       []string{"rds"},
				Regions:     []string{"us-east-1", "us-east-2", "us-west-2"},
			}}},
			Status: discoveryconfig.Status{
				LastSyncTime:        syncTime,
				DiscoveredResources: 2,
				IntegrationDiscoveredResources: map[string]*discoveryconfigv1.IntegrationDiscoveredSummary{
					integrationName: {
						AwsRds: &discoveryconfigv1.ResourcesDiscoveredSummary{Found: 2, Enrolled: 1, Failed: 1},
					},
				},
			},
		}
		dcForEKS := &discoveryconfig.DiscoveryConfig{
			Spec: discoveryconfig.Spec{AWS: []types.AWSMatcher{{
				Integration: integrationName,
				Types:       []string{"eks"},
				Regions:     []string{"us-east-1"},
			}}},
			Status: discoveryconfig.Status{
				LastSyncTime:        syncTime,
				DiscoveredResources: 2,
				IntegrationDiscoveredResources: map[string]*discoveryconfigv1.IntegrationDiscoveredSummary{
					integrationName: {
						AwsEks: &discoveryconfigv1.ResourcesDiscoveredSummary{Found: 4, Enrolled: 0, Failed: 0},
					},
				},
			},
		}
		clt := &mockRelevantAWSRegionsClient{
			discoveryConfigs: []*discoveryconfig.DiscoveryConfig{
				dcForEC2,
				dcForRDS,
				dcForEKS,
			},
			databaseServices: &proto.ListResourcesResponse{},
			databases:        make([]types.Database, 0),
		}

		gotSummary, err := collectAWSOIDCAutoDiscoverStats(ctx, logger, integration, clt, clt, deployedDatabaseServicesClient)
		require.NoError(t, err)
		expectedSummary := &ui.IntegrationWithSummary{
			Integration: &ui.Integration{
				Name:    integrationName,
				SubKind: "aws-oidc",
				AWSOIDC: &ui.IntegrationAWSOIDCSpec{RoleARN: "arn:role"},
			},
			AWSEC2: ui.ResourceTypeSummary{
				RulesCount:                 1,
				ResourcesFound:             2,
				ResourcesEnrollmentFailed:  1,
				ResourcesEnrollmentSuccess: 1,
				DiscoverLastSync:           &syncTime,
			},
			AWSRDS: ui.ResourceTypeSummary{
				RulesCount:                 3,
				ResourcesFound:             2,
				ResourcesEnrollmentFailed:  1,
				ResourcesEnrollmentSuccess: 1,
				ECSDatabaseServiceCount:    1,
				DiscoverLastSync:           &syncTime,
			},
			AWSEKS: ui.ResourceTypeSummary{
				RulesCount:                 1,
				ResourcesFound:             4,
				ResourcesEnrollmentFailed:  0,
				ResourcesEnrollmentSuccess: 0,
				DiscoverLastSync:           &syncTime,
			},
		}
		require.Equal(t, expectedSummary, gotSummary)
	})
}

func TestCollectAutoDiscoveryRules(t *testing.T) {
	ctx := context.Background()
	integrationName := "my-integration"

	t.Run("without discovery configs, returns no rules", func(t *testing.T) {
		clt := &mockRelevantAWSRegionsClient{
			discoveryConfigs: make([]*discoveryconfig.DiscoveryConfig, 0),
		}

		gotRules, err := collectAutoDiscoveryRules(ctx, integrationName, "", "", clt)
		require.NoError(t, err)
		expectedRules := ui.IntegrationDiscoveryRules{}
		require.Equal(t, expectedRules, gotRules)
	})

	t.Run("collects multiple discovery configs", func(t *testing.T) {
		syncTime := time.Now()
		dcForEC2 := &discoveryconfig.DiscoveryConfig{
			ResourceHeader: header.ResourceHeader{Metadata: header.Metadata{
				Name: uuid.NewString(),
			}},
			Spec: discoveryconfig.Spec{AWS: []types.AWSMatcher{{
				Integration: integrationName,
				Types:       []string{"ec2"},
				Regions:     []string{"us-east-1"},
				Tags:        types.Labels{"*": []string{"*"}},
			}}},
			Status: discoveryconfig.Status{
				LastSyncTime: syncTime,
			},
		}
		dcForRDS := &discoveryconfig.DiscoveryConfig{
			ResourceHeader: header.ResourceHeader{Metadata: header.Metadata{
				Name: uuid.NewString(),
			}},
			Spec: discoveryconfig.Spec{AWS: []types.AWSMatcher{{
				Integration: integrationName,
				Types:       []string{"rds"},
				Regions:     []string{"us-east-1", "us-east-2"},
				Tags: types.Labels{
					"env": []string{"dev", "prod"},
				},
			}}},
			Status: discoveryconfig.Status{
				LastSyncTime: syncTime,
			},
		}
		dcForEKS := &discoveryconfig.DiscoveryConfig{
			ResourceHeader: header.ResourceHeader{Metadata: header.Metadata{
				Name: uuid.NewString(),
			}},
			Spec: discoveryconfig.Spec{AWS: []types.AWSMatcher{{
				Integration: integrationName,
				Types:       []string{"eks"},
				Regions:     []string{"us-east-1"},
				Tags:        types.Labels{"*": []string{"*"}},
			}}},
			Status: discoveryconfig.Status{
				LastSyncTime: syncTime,
			},
		}
		dcForEKSWithoutStatus := &discoveryconfig.DiscoveryConfig{
			ResourceHeader: header.ResourceHeader{Metadata: header.Metadata{
				Name: uuid.NewString(),
			}},
			Spec: discoveryconfig.Spec{AWS: []types.AWSMatcher{{
				Integration: integrationName,
				Types:       []string{"eks"},
				Regions:     []string{"eu-west-1"},
				Tags:        types.Labels{"*": []string{"*"}},
			}}},
		}
		clt := &mockRelevantAWSRegionsClient{
			discoveryConfigs: []*discoveryconfig.DiscoveryConfig{
				dcForEC2,
				dcForRDS,
				dcForEKS,
				dcForEKSWithoutStatus,
			},
		}

		got, err := collectAutoDiscoveryRules(ctx, integrationName, "", "", clt)
		require.NoError(t, err)
		expectedRules := []ui.IntegrationDiscoveryRule{
			{
				ResourceType: "ec2",
				Region:       "us-east-1",
				LabelMatcher: []libui.Label{
					{Name: "*", Value: "*"},
				},
				DiscoveryConfig: dcForEC2.GetName(),
				LastSync:        &syncTime,
			},
			{
				ResourceType: "eks",
				Region:       "us-east-1",
				LabelMatcher: []libui.Label{
					{Name: "*", Value: "*"},
				},
				DiscoveryConfig: dcForEKS.GetName(),
				LastSync:        &syncTime,
			},
			{
				ResourceType: "eks",
				Region:       "eu-west-1",
				LabelMatcher: []libui.Label{
					{Name: "*", Value: "*"},
				},
				DiscoveryConfig: dcForEKSWithoutStatus.GetName(),
			},
			{
				ResourceType: "rds",
				Region:       "us-east-1",
				LabelMatcher: []libui.Label{
					{Name: "env", Value: "dev"},
					{Name: "env", Value: "prod"},
				},
				DiscoveryConfig: dcForRDS.GetName(),
				LastSync:        &syncTime,
			},
			{
				ResourceType: "rds",
				Region:       "us-east-2",
				LabelMatcher: []libui.Label{
					{Name: "env", Value: "dev"},
					{Name: "env", Value: "prod"},
				},
				DiscoveryConfig: dcForRDS.GetName(),
				LastSync:        &syncTime,
			},
		}
		require.Empty(t, got.NextKey)
		require.ElementsMatch(t, expectedRules, got.Rules)
	})
}
