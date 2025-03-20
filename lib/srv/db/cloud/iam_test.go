/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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
	"github.com/gravitational/teleport/lib/auth/authclient"
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

	memorydb, err := types.NewDatabaseV3(types.Metadata{
		Name: "aws-memorydb",
	}, types.DatabaseSpecV3{
		Protocol: "redis",
		URI:      "clustercfg.my-memorydb.xxxxxx.memorydb.us-east-1.amazonaws.com:6379",
		AWS: types.AWS{
			AccountID: "123456789012",
			MemoryDB: types.MemoryDB{
				ClusterName:  "my-memorydb",
				TLSEnabled:   true,
				EndpointType: "cluster",
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
		"MemoryDB": {
			database:           memorydb,
			wantPolicyContains: memorydb.GetAWS().MemoryDB.ClusterName,
			getIAMAuthEnabled: func() bool {
				return true // it always is for MemoryDB.
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
				// Initially unspecified since no tasks has ran yet.
				require.Equal(t, types.IAMPolicyStatus_IAM_POLICY_STATUS_UNSPECIFIED, database.GetAWS().IAMPolicyStatus)

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
				require.Equal(t, types.IAMPolicyStatus_IAM_POLICY_STATUS_SUCCESS, database.GetAWS().IAMPolicyStatus, "must be success because iam policy was set up")

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
				require.Equal(t, types.IAMPolicyStatus_IAM_POLICY_STATUS_UNSPECIFIED, database.GetAWS().IAMPolicyStatus, "must be unspecified because task is tearing down")
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
			require.Equal(t, types.IAMPolicyStatus_IAM_POLICY_STATUS_FAILED, database.GetAWS().IAMPolicyStatus, "must be invalid because of perm issues")

			err = configurator.processTask(ctx, iamTask{
				isSetup:  false,
				database: database,
			})
			require.NoError(t, err)

			err = configurator.UpdateIAMStatus(database)
			require.NoError(t, err)
			require.Equal(t, types.IAMPolicyStatus_IAM_POLICY_STATUS_UNSPECIFIED, database.GetAWS().IAMPolicyStatus, "must be unspecified, task is tearing down")
		})
	}
}

// mockAccessPoint is a mock for auth.DatabaseAccessPoint.
type mockAccessPoint struct {
	authclient.DatabaseAccessPoint
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
