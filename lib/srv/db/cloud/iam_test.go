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
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
)

// TestAWSIAM tests RDS, Aurora and Redshift IAM auto-configuration.
func TestAWSIAM(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// Setup AWS database objects.
	rdsInstance := &rds.DBInstance{
		DBInstanceArn:        aws.String("arn:aws:rds:us-west-1:1234567890:db:postgres-rds"),
		DBInstanceIdentifier: aws.String("postgres-rds"),
		DbiResourceId:        aws.String("db-xyz"),
	}

	auroraCluster := &rds.DBCluster{
		DBClusterArn:        aws.String("arn:aws:rds:us-east-1:1234567890:cluster:postgres-aurora"),
		DBClusterIdentifier: aws.String("postgres-aurora"),
		DbClusterResourceId: aws.String("cluster-xyz"),
	}

	redshiftCluster := &redshift.Cluster{
		ClusterNamespaceArn: aws.String("arn:aws:redshift:us-east-2:1234567890:namespace:namespace-xyz"),
		ClusterIdentifier:   aws.String("redshift-cluster-1"),
	}

	// Configure mocks.
	stsClient := &STSMock{
		ARN: "arn:aws:iam::1234567890:role/test-role",
	}

	rdsClient := &RDSMock{
		DBInstances: []*rds.DBInstance{rdsInstance},
		DBClusters:  []*rds.DBCluster{auroraCluster},
	}

	redshiftClient := &RedshiftMock{
		Clusters: []*redshift.Cluster{redshiftCluster},
	}

	iamClient := &IAMMock{}

	// Setup database resources.
	rdsDatabase, err := types.NewDatabaseV3(types.Metadata{
		Name: "postgres-rds",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost",
		AWS:      types.AWS{Region: "localhost", AccountID: "1234567890", RDS: types.RDS{InstanceID: "postgres-rds", ResourceID: "postgres-rds-resource-id"}},
	})
	require.NoError(t, err)

	auroraDatabase, err := types.NewDatabaseV3(types.Metadata{
		Name: "postgres-aurora",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost",
		AWS:      types.AWS{Region: "localhost", AccountID: "1234567890", RDS: types.RDS{ClusterID: "postgres-aurora", ResourceID: "postgres-aurora-resource-id"}},
	})
	require.NoError(t, err)

	rdsProxy, err := types.NewDatabaseV3(types.Metadata{
		Name: "rds-proxy",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost",
		AWS:      types.AWS{Region: "localhost", AccountID: "1234567890", RDSProxy: types.RDSProxy{Name: "rds-proxy", ResourceID: "rds-proxy-resource-id"}},
	})
	require.NoError(t, err)

	redshiftDatabase, err := types.NewDatabaseV3(types.Metadata{
		Name: "redshift",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost",
		AWS:      types.AWS{Region: "localhost", AccountID: "1234567890", Redshift: types.Redshift{ClusterID: "redshift-cluster-1"}},
	})
	require.NoError(t, err)

	databaseMissingMetadata, err := types.NewDatabaseV3(types.Metadata{
		Name: "redshift",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost",
		AWS:      types.AWS{Redshift: types.Redshift{ClusterID: "missing metadata"}},
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

	t.Run("RDS", func(t *testing.T) {
		// Configure RDS database and make sure IAM was enabled and policy was attached.
		err = configurator.Setup(ctx, rdsDatabase)
		require.NoError(t, err)
		waitForTaskProcessed(t)
		require.True(t, aws.BoolValue(rdsInstance.IAMDatabaseAuthenticationEnabled))
		policy := iamClient.attachedRolePolicies["test-role"][policyName]
		require.Contains(t, policy, rdsDatabase.GetAWS().RDS.ResourceID)

		// Deconfigure RDS database, policy should get detached.
		err = configurator.Teardown(ctx, rdsDatabase)
		require.NoError(t, err)
		waitForTaskProcessed(t)
		policy = iamClient.attachedRolePolicies["test-role"][policyName]
		require.NotContains(t, policy, rdsDatabase.GetAWS().RDS.ResourceID)
	})

	t.Run("Aurora", func(t *testing.T) {
		// Configure Aurora database and make sure IAM was enabled and policy was attached.
		err = configurator.Setup(ctx, auroraDatabase)
		require.NoError(t, err)
		waitForTaskProcessed(t)
		require.True(t, aws.BoolValue(auroraCluster.IAMDatabaseAuthenticationEnabled))
		policy := iamClient.attachedRolePolicies["test-role"][policyName]
		require.Contains(t, policy, auroraDatabase.GetAWS().RDS.ResourceID)

		// Deconfigure Aurora database, policy should get detached.
		err = configurator.Teardown(ctx, auroraDatabase)
		require.NoError(t, err)
		waitForTaskProcessed(t)
		policy = iamClient.attachedRolePolicies["test-role"][policyName]
		require.NotContains(t, policy, auroraDatabase.GetAWS().RDS.ResourceID)
	})

	t.Run("RDS Proxy", func(t *testing.T) {
		// Configure RDS Proxy database and make sure IAM was enabled and policy was attached.
		err = configurator.Setup(ctx, rdsProxy)
		require.NoError(t, err)
		waitForTaskProcessed(t)
		policy := iamClient.attachedRolePolicies["test-role"][policyName]
		require.Contains(t, policy, rdsProxy.GetAWS().RDSProxy.ResourceID)

		// Deconfigure RDS Proxy database, policy should get detached.
		err = configurator.Teardown(ctx, rdsProxy)
		require.NoError(t, err)
		waitForTaskProcessed(t)
		policy = iamClient.attachedRolePolicies["test-role"][policyName]
		require.NotContains(t, policy, rdsProxy.GetAWS().RDSProxy.ResourceID)
	})

	t.Run("Redshift", func(t *testing.T) {
		// Configure Redshift database and make sure policy was attached.
		err = configurator.Setup(ctx, redshiftDatabase)
		require.NoError(t, err)
		waitForTaskProcessed(t)
		policy := iamClient.attachedRolePolicies["test-role"][policyName]
		require.Contains(t, policy, redshiftDatabase.GetAWS().Redshift.ClusterID)

		// Deconfigure Redshift database, policy should get detached.
		err = configurator.Teardown(ctx, redshiftDatabase)
		require.NoError(t, err)
		waitForTaskProcessed(t)
		policy = iamClient.attachedRolePolicies["test-role"][policyName]
		require.NotContains(t, policy, redshiftDatabase.GetAWS().Redshift.ClusterID)
	})

	// Database misssing metadata for generating IAM actions should NOT be
	// added to the policy document.
	t.Run("missing metadata", func(t *testing.T) {
		err = configurator.Setup(ctx, databaseMissingMetadata)
		waitForTaskProcessed(t)
		policy := iamClient.attachedRolePolicies["test-role"][policyName]
		require.NotContains(t, policy, databaseMissingMetadata.GetAWS().Redshift.ClusterID)
	})
}

// TestAWSIAMNoPermissions tests that lack of AWS permissions does not produce
// errors during IAM auto-configuration.
func TestAWSIAMNoPermissions(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// Create unauthorized mocks for AWS services.
	stsClient := &STSMock{
		ARN: "arn:aws:iam::1234567890:role/test-role",
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
			meta: types.AWS{Region: "localhost", AccountID: "1234567890", RDS: types.RDS{InstanceID: "postgres-rds", ResourceID: "postgres-rds-resource-id"}},
			clients: &clients.TestCloudClients{
				RDS: &RDSMockUnauth{},
				IAM: &IAMErrorMock{
					Error: trace.AccessDenied("unauthorized"),
				},
				STS: stsClient,
			},
		},
		{
			name: "Aurora cluster",
			meta: types.AWS{Region: "localhost", AccountID: "1234567890", RDS: types.RDS{ClusterID: "postgres-aurora", ResourceID: "postgres-aurora-resource-id"}},
			clients: &clients.TestCloudClients{
				RDS: &RDSMockUnauth{},
				IAM: &IAMErrorMock{
					Error: trace.AccessDenied("unauthorized"),
				},
				STS: stsClient,
			},
		},
		{
			name: "Redshift cluster",
			meta: types.AWS{Region: "localhost", AccountID: "1234567890", Redshift: types.Redshift{ClusterID: "redshift-cluster-1"}},
			clients: &clients.TestCloudClients{
				Redshift: &RedshiftMockUnauth{},
				IAM: &IAMErrorMock{
					Error: trace.AccessDenied("unauthorized"),
				},
				STS: stsClient,
			},
		},
		{
			name: "IAM UnmodifiableEntityException",
			meta: types.AWS{Region: "localhost", AccountID: "1234567890", Redshift: types.Redshift{ClusterID: "redshift-cluster-1"}},
			clients: &clients.TestCloudClients{
				Redshift: &RedshiftMockUnauth{},
				IAM: &IAMErrorMock{
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

			err = configurator.processTask(ctx, iamTask{
				isSetup:  false,
				database: database,
			})
			require.NoError(t, err)
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
