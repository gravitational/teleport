/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cloud

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/redshift"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	clients "github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/cloud/mocks"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
)

// TestAWSIAM tests RDS, Aurora and Redshift IAM auto-configuration.
func TestAWSIAM(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// Setup AWS database objects.
	rdsInstance := &rds.DBInstance{
		DBInstanceArn:        aws.String("arn:aws:rds:us-west-1:123456789012:db:postgres-rds"),
		DBInstanceIdentifier: aws.String("postgres-rds"),
		DbiResourceId:        aws.String("db-xyz"),
	}

	auroraCluster := &rds.DBCluster{
		DBClusterArn:        aws.String("arn:aws:rds:us-east-1:123456789012:cluster:postgres-aurora"),
		DBClusterIdentifier: aws.String("postgres-aurora"),
		DbClusterResourceId: aws.String("cluster-xyz"),
	}

	redshiftCluster := &redshift.Cluster{
		ClusterNamespaceArn: aws.String("arn:aws:redshift:us-east-2:123456789012:namespace:namespace-xyz"),
		ClusterIdentifier:   aws.String("redshift-cluster-1"),
	}

	// Configure mocks.
	stsClient := &mocks.STSMock{
		ARN: "arn:aws:iam::123456789012:role/test-role",
	}

	rdsClient := &mocks.RDSMock{
		DBInstances: []*rds.DBInstance{rdsInstance},
		DBClusters:  []*rds.DBCluster{auroraCluster},
	}

	redshiftClient := &mocks.RedshiftMock{
		Clusters: []*redshift.Cluster{redshiftCluster},
	}

	iamClient := &mocks.IAMMock{}

	// Setup database resources.
	rdsDatabase, err := types.NewDatabaseV3(types.Metadata{
		Name: "postgres-rds",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost",
		AWS:      types.AWS{Region: "localhost", AccountID: "123456789012", RDS: types.RDS{InstanceID: "postgres-rds", ResourceID: "postgres-rds-resource-id"}},
	})
	require.NoError(t, err)

	auroraDatabase, err := types.NewDatabaseV3(types.Metadata{
		Name: "postgres-aurora",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost",
		AWS:      types.AWS{Region: "localhost", AccountID: "123456789012", RDS: types.RDS{ClusterID: "postgres-aurora", ResourceID: "postgres-aurora-resource-id"}},
	})
	require.NoError(t, err)

	rdsProxy, err := types.NewDatabaseV3(types.Metadata{
		Name: "rds-proxy",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost",
		AWS:      types.AWS{Region: "localhost", AccountID: "123456789012", RDSProxy: types.RDSProxy{Name: "rds-proxy", ResourceID: "rds-proxy-resource-id"}},
	})
	require.NoError(t, err)

	redshiftDatabase, err := types.NewDatabaseV3(types.Metadata{
		Name: "redshift",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost",
		AWS:      types.AWS{Region: "localhost", AccountID: "123456789012", Redshift: types.Redshift{ClusterID: "redshift-cluster-1"}},
	})
	require.NoError(t, err)

	elasticache, err := types.NewDatabaseV3(types.Metadata{
		Name: "aws-elasticache",
	}, types.DatabaseSpecV3{
		Protocol: "redis",
		URI:      "clustercfg.my-redis-cluster.xxxxxx.cac1.cache.amazonaws.com:6379",
		AWS: types.AWS{
			AccountID: "123456789012",
			ElastiCache: types.ElastiCache{
				ReplicationGroupID: "some-group",
			},
		},
	})
	require.NoError(t, err)

	// Make configurator.
	taskChan := make(chan struct{})
	waitForTaskProcessed := func(t *testing.T) {
		select {
		case <-taskChan:
		case <-time.After(5 * time.Second):
			require.Fail(t, "Failed to wait for task is processed")
		}
	}
	assumedRole := types.AssumeRole{
		RoleARN:    "arn:aws:iam::123456789012:role/role-to-assume",
		ExternalID: "externalid123",
	}
	configurator, err := NewIAM(ctx, IAMConfig{
		AccessPoint: &mockAccessPoint{},
		Clients: &clients.TestCloudClients{
			RDS:      rdsClient,
			Redshift: redshiftClient,
			STS:      stsClient,
			IAM:      iamClient,
		},
		HostID: "host-id",
		onProcessedTask: func(iamTask, error) {
			taskChan <- struct{}{}
		},
	})
	require.NoError(t, err)
	require.NoError(t, configurator.Start(ctx))

	policyName, err := configurator.getPolicyName()
	require.NoError(t, err)

	tests := map[string]struct {
		database           types.Database
		wantPolicyContains string
		getIAMAuthEnabled  func() bool
	}{
		"RDS": {
			database:           rdsDatabase,
			wantPolicyContains: rdsDatabase.GetAWS().RDS.ResourceID,
			getIAMAuthEnabled: func() bool {
				out := aws.BoolValue(rdsInstance.IAMDatabaseAuthenticationEnabled)
				// reset it
				rdsInstance.IAMDatabaseAuthenticationEnabled = aws.Bool(false)
				return out
			},
		},
		"Aurora": {
			database:           auroraDatabase,
			wantPolicyContains: auroraDatabase.GetAWS().RDS.ResourceID,
			getIAMAuthEnabled: func() bool {
				out := aws.BoolValue(auroraCluster.IAMDatabaseAuthenticationEnabled)
				// reset it
				auroraCluster.IAMDatabaseAuthenticationEnabled = aws.Bool(false)
				return out
			},
		},
		"RDS Proxy": {
			database:           rdsProxy,
			wantPolicyContains: rdsProxy.GetAWS().RDSProxy.ResourceID,
			getIAMAuthEnabled: func() bool {
				return true // it always is for rds proxy.
			},
		},
		"Redshift": {
			database:           redshiftDatabase,
			wantPolicyContains: redshiftDatabase.GetAWS().Redshift.ClusterID,
			getIAMAuthEnabled: func() bool {
				return true // it always is for redshift.
			},
		},
		"ElastiCache": {
			database:           elasticache,
			wantPolicyContains: elasticache.GetAWS().ElastiCache.ReplicationGroupID,
			getIAMAuthEnabled: func() bool {
				return true // it always is for ElastiCache.
			},
		},
	}

	for testName, tt := range tests {
		for _, assumeRole := range []types.AssumeRole{{}, assumedRole} {
			getRolePolicyInput := &iam.GetRolePolicyInput{
				RoleName:   aws.String("test-role"),
				PolicyName: aws.String(policyName),
			}
			database := tt.database.Copy()
			if assumeRole.RoleARN != "" {
				testName += " with assumed role"
				getRolePolicyInput.RoleName = aws.String("role-to-assume")
				meta := database.GetAWS()
				meta.AssumeRoleARN = assumeRole.RoleARN
				meta.ExternalID = assumeRole.ExternalID
				database.SetStatusAWS(meta)
			}
			t.Run(testName, func(t *testing.T) {
				// Configure database and make sure IAM is enabled and policy was attached.
				err = configurator.Setup(ctx, database)
				require.NoError(t, err)
				waitForTaskProcessed(t)
				output, err := iamClient.GetRolePolicyWithContext(ctx, getRolePolicyInput)
				require.NoError(t, err)
				require.True(t, tt.getIAMAuthEnabled())
				require.Contains(t, aws.StringValue(output.PolicyDocument), tt.wantPolicyContains)

				err = configurator.UpdateIAMStatus(database)
				require.NoError(t, err)
				require.True(t, database.GetAWS().IAMPolicyExists, "must be true because iam policy was set up")

				// Deconfigure database, policy should get detached.
				err = configurator.Teardown(ctx, database)
				require.NoError(t, err)
				waitForTaskProcessed(t)
				_, err = iamClient.GetRolePolicyWithContext(ctx, getRolePolicyInput)
				require.True(t, trace.IsNotFound(err))
				meta := database.GetAWS()
				if meta.AssumeRoleARN != "" {
					require.Equal(t, []string{meta.AssumeRoleARN}, stsClient.GetAssumedRoleARNs())
					require.Equal(t, []string{meta.ExternalID}, stsClient.GetAssumedRoleExternalIDs())
					stsClient.ResetAssumeRoleHistory()
				}

				err = configurator.UpdateIAMStatus(database)
				require.NoError(t, err)
				require.False(t, database.GetAWS().IAMPolicyExists, "must be false because iam policy was removed")
			})
		}
	}
}

// TestAWSIAMNoPermissions tests that lack of AWS permissions does not produce
// errors during IAM auto-configuration.
func TestAWSIAMNoPermissions(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// Create unauthorized mocks for AWS services.
	stsClient := &mocks.STSMock{
		ARN: "arn:aws:iam::123456789012:role/test-role",
	}
	// Make configurator.
	configurator, err := NewIAM(ctx, IAMConfig{
		AccessPoint: &mockAccessPoint{},
		Clients:     &clients.TestCloudClients{}, // placeholder,
		HostID:      "host-id",
	})
	require.NoError(t, err)

	tests := []struct {
		name    string
		meta    types.AWS
		clients clients.Clients
	}{
		{
			name: "RDS database",
			meta: types.AWS{Region: "localhost", AccountID: "123456789012", RDS: types.RDS{InstanceID: "postgres-rds", ResourceID: "postgres-rds-resource-id"}},
			clients: &clients.TestCloudClients{
				RDS: &mocks.RDSMockUnauth{},
				IAM: &mocks.IAMErrorMock{
					Error: trace.AccessDenied("unauthorized"),
				},
				STS: stsClient,
			},
		},
		{
			name: "Aurora cluster",
			meta: types.AWS{Region: "localhost", AccountID: "123456789012", RDS: types.RDS{ClusterID: "postgres-aurora", ResourceID: "postgres-aurora-resource-id"}},
			clients: &clients.TestCloudClients{
				RDS: &mocks.RDSMockUnauth{},
				IAM: &mocks.IAMErrorMock{
					Error: trace.AccessDenied("unauthorized"),
				},
				STS: stsClient,
			},
		},
		{
			name: "RDS database missing metadata",
			meta: types.AWS{Region: "localhost", RDS: types.RDS{ClusterID: "postgres-aurora"}},
			clients: &clients.TestCloudClients{
				RDS: &mocks.RDSMockUnauth{},
				IAM: &mocks.IAMErrorMock{
					Error: trace.AccessDenied("unauthorized"),
				},
				STS: stsClient,
			},
		},
		{
			name: "Redshift cluster",
			meta: types.AWS{Region: "localhost", AccountID: "123456789012", Redshift: types.Redshift{ClusterID: "redshift-cluster-1"}},
			clients: &clients.TestCloudClients{
				Redshift: &mocks.RedshiftMockUnauth{},
				IAM: &mocks.IAMErrorMock{
					Error: trace.AccessDenied("unauthorized"),
				},
				STS: stsClient,
			},
		},
		{
			name: "ElastiCache",
			meta: types.AWS{Region: "localhost", AccountID: "123456789012", ElastiCache: types.ElastiCache{ReplicationGroupID: "some-group"}},
			clients: &clients.TestCloudClients{
				// As of writing this API won't be called by the configurator anyway,
				// but might as well provide it in case that changes.
				ElastiCache: &mocks.ElastiCacheMock{Unauth: true},
				IAM: &mocks.IAMErrorMock{
					Error: trace.AccessDenied("unauthorized"),
				},
				STS: stsClient,
			},
		},
		{
			name: "IAM UnmodifiableEntityException",
			meta: types.AWS{Region: "localhost", AccountID: "123456789012", Redshift: types.Redshift{ClusterID: "redshift-cluster-1"}},
			clients: &clients.TestCloudClients{
				Redshift: &mocks.RedshiftMockUnauth{},
				IAM: &mocks.IAMErrorMock{
					Error: awserr.New(iam.ErrCodeUnmodifiableEntityException, "unauthorized", fmt.Errorf("unauthorized")),
				},
				STS: stsClient,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Update cloud clients.
			configurator.cfg.Clients = test.clients

			database, err := types.NewDatabaseV3(types.Metadata{
				Name: "test",
			}, types.DatabaseSpecV3{
				Protocol: defaults.ProtocolPostgres,
				URI:      "localhost",
				AWS:      test.meta,
			})
			require.NoError(t, err)

			// Make sure there're no errors trying to setup/destroy IAM.
			err = configurator.processTask(ctx, iamTask{
				isSetup:  true,
				database: database,
			})
			require.NoError(t, err)

			err = configurator.UpdateIAMStatus(database)
			require.NoError(t, err)
			require.False(t, database.GetAWS().IAMPolicyExists, "iam policy was not created, should return false")

			err = configurator.processTask(ctx, iamTask{
				isSetup:  false,
				database: database,
			})
			require.NoError(t, err)

			err = configurator.UpdateIAMStatus(database)
			require.NoError(t, err)
			require.False(t, database.GetAWS().IAMPolicyExists, "iam policy was not created, should return false")
		})
	}
}

// mockAccessPoint is a mock for auth.DatabaseAccessPoint.
type mockAccessPoint struct {
	auth.DatabaseAccessPoint
}

func (m *mockAccessPoint) GetClusterName(opts ...services.MarshalOption) (types.ClusterName, error) {
	return types.NewClusterName(types.ClusterNameSpecV2{
		ClusterName: "cluster.local",
		ClusterID:   "cluster-id",
	})
}
func (m *mockAccessPoint) AcquireSemaphore(ctx context.Context, params types.AcquireSemaphoreRequest) (*types.SemaphoreLease, error) {
	return &types.SemaphoreLease{
		SemaphoreKind: params.SemaphoreKind,
		SemaphoreName: params.SemaphoreName,
		LeaseID:       uuid.NewString(),
		Expires:       params.Expires,
	}, nil
}
func (m *mockAccessPoint) CancelSemaphoreLease(ctx context.Context, lease types.SemaphoreLease) error {
	return nil
}
