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

package awsoidc

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecsTypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	awsV1Http "github.com/aws/smithy-go/transport/http"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/aws/tags"
)

func TestDeployDatabaseServiceRequest_CheckAndSetDefaults(t *testing.T) {
	isBadParamErrFn := func(tt require.TestingT, err error, i ...any) {
		require.True(tt, trace.IsBadParameter(err), "expected bad parameter, got %v", err)
	}

	baseReqFn := func() DeployDatabaseServiceRequest {
		return DeployDatabaseServiceRequest{
			TeleportClusterName: "mycluster",
			Region:              "r",
			TaskRoleARN:         "arn",
			IntegrationName:     "teleportdev",
			Deployments: []DeployDatabaseServiceRequestDeployment{{
				VPCID:               "vpc-123",
				SubnetIDs:           []string{"subnet-1", "subnet-2"},
				SecurityGroupIDs:    []string{"sg-1", "sg-2"},
				DeployServiceConfig: "teleport.yaml-base64",
			}},
			DeploymentJoinTokenName: "discover-aws-oidc-iam-token",
		}
	}

	for _, tt := range []struct {
		name     string
		req      func() DeployDatabaseServiceRequest
		errCheck require.ErrorAssertionFunc
		expected *DeployDatabaseServiceRequest
	}{
		{
			name: "no fields",
			req: func() DeployDatabaseServiceRequest {
				return DeployDatabaseServiceRequest{}
			},
			errCheck: isBadParamErrFn,
		},
		{
			name: "missing teleport cluster name",
			req: func() DeployDatabaseServiceRequest {
				r := baseReqFn()
				r.TeleportClusterName = ""
				return r
			},
			errCheck: isBadParamErrFn,
		},
		{
			name: "missing region",
			req: func() DeployDatabaseServiceRequest {
				r := baseReqFn()
				r.Region = ""
				return r
			},
			errCheck: isBadParamErrFn,
		},
		{
			name: "missing task role",
			req: func() DeployDatabaseServiceRequest {
				r := baseReqFn()
				r.TaskRoleARN = ""
				return r
			},
			errCheck: isBadParamErrFn,
		},
		{
			name: "missing integration name",
			req: func() DeployDatabaseServiceRequest {
				r := baseReqFn()
				r.IntegrationName = ""
				return r
			},
			errCheck: isBadParamErrFn,
		},
		{
			name: "empty list of subnets",
			req: func() DeployDatabaseServiceRequest {
				r := baseReqFn()
				r.Deployments[0].SubnetIDs = []string{}
				return r
			},
			errCheck: isBadParamErrFn,
		},
		{
			name: "empty teleport config",
			req: func() DeployDatabaseServiceRequest {
				r := baseReqFn()
				r.Deployments[0].DeployServiceConfig = ""
				return r
			},
			errCheck: isBadParamErrFn,
		},
		{
			name: "empty vpc id",
			req: func() DeployDatabaseServiceRequest {
				r := baseReqFn()
				r.Deployments[0].VPCID = ""
				return r
			},
			errCheck: isBadParamErrFn,
		},
		{
			name: "empty deployment list",
			req: func() DeployDatabaseServiceRequest {
				r := baseReqFn()
				r.Deployments = []DeployDatabaseServiceRequestDeployment{}
				return r
			},
			errCheck: isBadParamErrFn,
		},
		{
			name:     "fill defaults",
			req:      baseReqFn,
			errCheck: require.NoError,
			expected: &DeployDatabaseServiceRequest{
				TeleportClusterName: "mycluster",
				TeleportVersionTag:  teleport.Version,
				Region:              "r",
				TaskRoleARN:         "arn",
				IntegrationName:     "teleportdev",
				ResourceCreationTags: tags.AWSTags{
					"teleport.dev/origin":      "integration_awsoidc",
					"teleport.dev/cluster":     "mycluster",
					"teleport.dev/integration": "teleportdev",
				},
				Deployments: []DeployDatabaseServiceRequestDeployment{{
					VPCID:               "vpc-123",
					SubnetIDs:           []string{"subnet-1", "subnet-2"},
					SecurityGroupIDs:    []string{"sg-1", "sg-2"},
					DeployServiceConfig: "teleport.yaml-base64",
				}},
				ecsClusterName:          "mycluster-teleport",
				DeploymentJoinTokenName: "discover-aws-oidc-iam-token",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := tt.req()
			err := r.CheckAndSetDefaults()
			tt.errCheck(t, err)

			if err != nil {
				return
			}
			if tt.expected != nil {
				require.Empty(t, cmp.Diff(
					*tt.expected,
					r,
					cmpopts.IgnoreUnexported(DeployDatabaseServiceRequest{}),
				))
			}
		})
	}
}

type mockDeployServiceClient struct {
	mu              sync.RWMutex
	clusters        map[string]*ecsTypes.Cluster
	taskDefinitions map[string]*ecsTypes.TaskDefinition
	services        map[string]*ecsTypes.Service

	accountId       *string
	iamTokenMissing bool

	iamAccessDeniedListServices bool
	defaultTags                 tags.AWSTags
}

// DescribeClusters lists ECS Clusters.
// https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/ecs@v1.27.1#Client.DescribeClusters
func (m *mockDeployServiceClient) DescribeClusters(ctx context.Context, params *ecs.DescribeClustersInput, optFns ...func(*ecs.Options)) (*ecs.DescribeClustersOutput, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ret := []ecsTypes.Cluster{}

	if cluster, found := m.clusters[params.Clusters[0]]; found {
		ret = append(ret, *cluster)
	}

	return &ecs.DescribeClustersOutput{
		Clusters: ret,
	}, nil
}

// CreateCluster creates a new cluster.
// https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/ecs@v1.27.1#Client.CreateCluster
func (m *mockDeployServiceClient) CreateCluster(ctx context.Context, params *ecs.CreateClusterInput, optFns ...func(*ecs.Options)) (*ecs.CreateClusterOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cluster := &ecs.CreateClusterOutput{
		Cluster: &ecsTypes.Cluster{
			Status:      aws.String("ACTIVE"),
			ClusterName: params.ClusterName,
			ClusterArn:  aws.String("ARN" + aws.ToString(params.ClusterName)),
		},
	}
	m.clusters[aws.ToString(params.ClusterName)] = cluster.Cluster
	return cluster, nil
}

// PutClusterCapacityProviders sets the Capacity Providers available for services in a given cluster.
// https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/ecs@v1.27.1#Client.PutClusterCapacityProviders
func (m *mockDeployServiceClient) PutClusterCapacityProviders(ctx context.Context, params *ecs.PutClusterCapacityProvidersInput, optFns ...func(*ecs.Options)) (*ecs.PutClusterCapacityProvidersOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return &ecs.PutClusterCapacityProvidersOutput{
		Cluster: &ecsTypes.Cluster{
			Status:      aws.String("ACTIVE"),
			ClusterName: params.Cluster,
			ClusterArn:  aws.String("ARN" + aws.ToString(params.Cluster)),
		},
	}, nil
}

// DescribeServices lists the matching Services of a given Cluster.
// https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/ecs@v1.27.1#Client.DescribeServices
func (m *mockDeployServiceClient) DescribeServices(ctx context.Context, params *ecs.DescribeServicesInput, optFns ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ret := []ecsTypes.Service{}

	for _, serviceName := range params.Services {
		if service, found := m.services[serviceName]; found {
			ret = append(ret, *service)
		}
	}

	return &ecs.DescribeServicesOutput{
		Services: ret,
	}, nil
}

// ListServices returns a list of services. You can filter the results by cluster, launch type,
// and scheduling strategy.
func (m *mockDeployServiceClient) ListServices(ctx context.Context, params *ecs.ListServicesInput, optFns ...func(*ecs.Options)) (*ecs.ListServicesOutput, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.iamAccessDeniedListServices {
		return nil, &awshttp.ResponseError{
			ResponseError: &awsV1Http.ResponseError{
				Response: &awsV1Http.Response{
					Response: &http.Response{
						StatusCode: http.StatusBadRequest,
					},
				},
				Err: fmt.Errorf("AccessDeniedException"),
			},
			RequestID: uuid.NewString(),
		}
	}

	ret := []string{}

	for _, service := range m.services {
		if aws.ToString(service.ClusterArn) == aws.ToString(params.Cluster) {
			ret = append(ret, aws.ToString(service.ServiceArn))
		}
	}

	return &ecs.ListServicesOutput{
		ServiceArns: ret,
	}, nil
}

// UpdateService updates the service.
// https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/ecs@v1.27.1#Client.UpdateService
func (m *mockDeployServiceClient) UpdateService(ctx context.Context, params *ecs.UpdateServiceInput, optFns ...func(*ecs.Options)) (*ecs.UpdateServiceOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ret := &ecsTypes.Service{
		ServiceName:          params.Service,
		ServiceArn:           aws.String("ARN" + aws.ToString(params.Service)),
		NetworkConfiguration: params.NetworkConfiguration,
		Status:               aws.String("ACTIVE"),
		Deployments:          []ecsTypes.Deployment{{}},
		DesiredCount:         1,
		RunningCount:         1,
	}
	m.services[aws.ToString(params.Service)] = ret

	return &ecs.UpdateServiceOutput{
		Service: ret,
	}, nil
}

// CreateService starts a task within a cluster.
// https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/ecs@v1.27.1#Client.CreateService
func (m *mockDeployServiceClient) CreateService(ctx context.Context, params *ecs.CreateServiceInput, optFns ...func(*ecs.Options)) (*ecs.CreateServiceOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	service := &ecs.CreateServiceOutput{
		Service: &ecsTypes.Service{
			ServiceName:          params.ServiceName,
			NetworkConfiguration: params.NetworkConfiguration,
		},
	}

	m.services[aws.ToString(service.Service.ServiceName)] = service.Service

	return service, nil
}

// DescribeTaskDefinition describes the task definition.
// https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/ecs@v1.27.1#Client.DescribeTaskDefinition
func (m *mockDeployServiceClient) DescribeTaskDefinition(ctx context.Context, params *ecs.DescribeTaskDefinitionInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTaskDefinitionOutput, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, taskDef := range m.taskDefinitions {
		if aws.ToString(taskDef.Family) == aws.ToString(params.TaskDefinition) {
			return &ecs.DescribeTaskDefinitionOutput{
				TaskDefinition: taskDef,
				Tags:           m.defaultTags.ToECSTags(),
			}, nil
		}
	}
	return nil, trace.NotFound("not found")
}

// RegisterTaskDefinition registers a new task definition from the supplied family and containerDefinitions.
// https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/ecs@v1.27.1#Client.RegisterTaskDefinition
func (m *mockDeployServiceClient) RegisterTaskDefinition(ctx context.Context, params *ecs.RegisterTaskDefinitionInput, optFns ...func(*ecs.Options)) (*ecs.RegisterTaskDefinitionOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	taskDef := &ecs.RegisterTaskDefinitionOutput{
		TaskDefinition: &ecsTypes.TaskDefinition{
			TaskDefinitionArn:    aws.String("arn-for-task-definition==" + aws.ToString(params.Family)),
			ContainerDefinitions: params.ContainerDefinitions,
		},
	}
	m.taskDefinitions[aws.ToString(params.Family)] = taskDef.TaskDefinition

	return taskDef, nil
}

// DeregisterTaskDefinition deregisters the task definition.
// https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/ecs@v1.27.1#Client.DeregisterTaskDefinition
func (m *mockDeployServiceClient) DeregisterTaskDefinition(ctx context.Context, params *ecs.DeregisterTaskDefinitionInput, optFns ...func(*ecs.Options)) (*ecs.DeregisterTaskDefinitionOutput, error) {
	return nil, nil
}

// GetCallerIdentity returns details about the IAM user or role whose credentials are used to call the operation.
func (m *mockDeployServiceClient) GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
	return &sts.GetCallerIdentityOutput{
		Account: m.accountId,
	}, nil
}

// GetToken returns a provision token by name.
func (m *mockDeployServiceClient) GetToken(ctx context.Context, name string) (types.ProvisionToken, error) {
	return nil, trace.NotFound("token not found")
}

// UpsertToken creates or updates a provision token.
func (m *mockDeployServiceClient) UpsertToken(ctx context.Context, token types.ProvisionToken) error {
	return nil
}

func TestDeployDatabaseService(t *testing.T) {
	ctx := context.Background()
	t.Run("fails to get account id", func(t *testing.T) {
		_, err := DeployDatabaseService(ctx, &mockDeployServiceClient{}, DeployDatabaseServiceRequest{
			Region:              "us-east-1",
			TaskRoleARN:         "my-role",
			TeleportClusterName: "cluster-name",
			IntegrationName:     "my-integration",
			Deployments: []DeployDatabaseServiceRequestDeployment{
				{
					VPCID:               "vpc-123",
					SubnetIDs:           []string{"subnet-1", "subnet-2"},
					DeployServiceConfig: "teleport.yaml-base64",
				},
			},
		})
		require.True(t, trace.IsBadParameter(err), "expected bad parameter, got %+v", err)
	})
	t.Run("nothing exists", func(t *testing.T) {
		mockClient := &mockDeployServiceClient{
			clusters:        map[string]*ecsTypes.Cluster{},
			taskDefinitions: map[string]*ecsTypes.TaskDefinition{},
			services:        map[string]*ecsTypes.Service{},
			accountId:       aws.String("123456789012"),
			iamTokenMissing: true,
		}
		resp, err := DeployDatabaseService(ctx,
			mockClient,
			DeployDatabaseServiceRequest{
				Region:                  "us-east-1",
				TaskRoleARN:             "my-role",
				TeleportClusterName:     "cluster-name",
				IntegrationName:         "my-integration",
				DeploymentJoinTokenName: "discover-aws-oidc-iam-token",
				Deployments: []DeployDatabaseServiceRequestDeployment{
					{
						VPCID:               "vpc-123",
						SubnetIDs:           []string{"subnet-1", "subnet-2"},
						DeployServiceConfig: "teleport.yaml-base64",
					},
				},
			},
		)
		require.NoError(t, err)
		require.Equal(t, "https://us-east-1.console.aws.amazon.com/ecs/v2/clusters/cluster-name-teleport/services/database-service-vpc-123", resp.ClusterDashboardURL)
		require.Equal(t, "ARNcluster-name-teleport", resp.ClusterARN)
		require.Contains(t, mockClient.clusters, "cluster-name-teleport")
		require.Contains(t, mockClient.services, "database-service-vpc-123")
		require.Contains(t, mockClient.taskDefinitions, "cluster-name-teleport-database-service-vpc-123")
	})

	t.Run("nothing exists, multiple deployments", func(t *testing.T) {
		mockClient := &mockDeployServiceClient{
			clusters:        map[string]*ecsTypes.Cluster{},
			taskDefinitions: map[string]*ecsTypes.TaskDefinition{},
			services:        map[string]*ecsTypes.Service{},
			accountId:       aws.String("123456789012"),
			iamTokenMissing: true,
		}
		resp, err := DeployDatabaseService(ctx,
			mockClient,
			DeployDatabaseServiceRequest{
				Region:                  "us-east-1",
				TaskRoleARN:             "my-role",
				TeleportClusterName:     "cluster-name",
				IntegrationName:         "my-integration",
				DeploymentJoinTokenName: "discover-aws-oidc-iam-token",
				Deployments: []DeployDatabaseServiceRequestDeployment{
					{
						VPCID:               "vpc-001",
						SubnetIDs:           []string{"subnet-1", "subnet-2"},
						DeployServiceConfig: "teleport.yaml-base64",
					},
					{
						VPCID:               "vpc-002",
						SubnetIDs:           []string{"subnet-1", "subnet-2"},
						DeployServiceConfig: "teleport.yaml-base64",
					},
					{
						VPCID:               "vpc-003",
						SubnetIDs:           []string{"subnet-1", "subnet-2"},
						DeployServiceConfig: "teleport.yaml-base64",
					},
					{
						VPCID:               "vpc-004",
						SubnetIDs:           []string{"subnet-1", "subnet-2"},
						DeployServiceConfig: "teleport.yaml-base64",
					},
				},
			},
		)
		require.NoError(t, err)

		require.Equal(t, "https://us-east-1.console.aws.amazon.com/ecs/v2/clusters/cluster-name-teleport/services", resp.ClusterDashboardURL)
		require.Equal(t, "ARNcluster-name-teleport", resp.ClusterARN)
		require.Contains(t, mockClient.clusters, "cluster-name-teleport")

		require.Contains(t, mockClient.services, "database-service-vpc-001")
		require.Contains(t, mockClient.taskDefinitions, "cluster-name-teleport-database-service-vpc-001")
		require.Contains(t, mockClient.services, "database-service-vpc-002")
		require.Contains(t, mockClient.taskDefinitions, "cluster-name-teleport-database-service-vpc-002")
		require.Contains(t, mockClient.services, "database-service-vpc-003")
		require.Contains(t, mockClient.taskDefinitions, "cluster-name-teleport-database-service-vpc-003")
		require.Contains(t, mockClient.services, "database-service-vpc-004")
		require.Contains(t, mockClient.taskDefinitions, "cluster-name-teleport-database-service-vpc-004")

		serviceNetworkConfig := mockClient.services["database-service-vpc-001"].NetworkConfiguration.AwsvpcConfiguration
		require.ElementsMatch(t, serviceNetworkConfig.Subnets, []string{"subnet-1", "subnet-2"})
	})

	t.Run("ecs cluster and service already exist", func(t *testing.T) {
		resp, err := DeployDatabaseService(ctx,
			&mockDeployServiceClient{
				clusters: map[string]*ecsTypes.Cluster{
					"cluster-name-teleport": {
						ClusterArn: aws.String("abc"),
						Status:     aws.String("ACTIVE"),
						Tags: []ecsTypes.Tag{
							{Key: aws.String("teleport.dev/origin"), Value: aws.String("integration_awsoidc")},
							{Key: aws.String("teleport.dev/cluster"), Value: aws.String("cluster-name")},
							{Key: aws.String("teleport.dev/integration"), Value: aws.String("my-integration")},
						},
					},
				},
				taskDefinitions: map[string]*ecsTypes.TaskDefinition{},
				services: map[string]*ecsTypes.Service{
					"database-service-vpc-123": {
						Status:     aws.String("ACTIVE"),
						LaunchType: ecsTypes.LaunchTypeFargate,
						Tags: []ecsTypes.Tag{
							{Key: aws.String("teleport.dev/origin"), Value: aws.String("integration_awsoidc")},
							{Key: aws.String("teleport.dev/cluster"), Value: aws.String("cluster-name")},
							{Key: aws.String("teleport.dev/integration"), Value: aws.String("my-integration")},
						},
					},
				},
				accountId:       aws.String("123456789012"),
				iamTokenMissing: true,
			},
			DeployDatabaseServiceRequest{
				Region:                  "us-east-1",
				TaskRoleARN:             "my-role",
				TeleportClusterName:     "cluster-name",
				IntegrationName:         "my-integration",
				DeploymentJoinTokenName: "discover-aws-oidc-iam-token",
				Deployments: []DeployDatabaseServiceRequestDeployment{
					{
						VPCID:               "vpc-123",
						SubnetIDs:           []string{"subnet-1", "subnet-2"},
						DeployServiceConfig: "teleport.yaml-base64",
					},
				},
			},
		)
		require.NoError(t, err)
		require.Equal(t, "https://us-east-1.console.aws.amazon.com/ecs/v2/clusters/cluster-name-teleport/services/database-service-vpc-123", resp.ClusterDashboardURL)
		require.Equal(t, "ARNcluster-name-teleport", resp.ClusterARN)
	})
}

func TestECSDatabaseServiceDashboardURL(t *testing.T) {
	tests := []struct {
		name                string
		region              string
		teleportClusterName string
		vpcID               string
		wantURL             string
		wantErrContains     string
	}{
		{
			name:                "valid params",
			region:              "us-west-1",
			teleportClusterName: "foo.bar.com",
			vpcID:               "vpc-123",
			wantURL:             "https://us-west-1.console.aws.amazon.com/ecs/v2/clusters/foo_bar_com-teleport/services/database-service-vpc-123",
		},
		{
			name:                "empty region is an error",
			region:              "",
			teleportClusterName: "foo.bar.com",
			vpcID:               "vpc-123",
			wantErrContains:     "empty region",
		},
		{
			name:                "empty cluster name is an error",
			region:              "us-west-1",
			teleportClusterName: "",
			vpcID:               "vpc-123",
			wantErrContains:     "empty cluster name",
		},
		{
			name:                "empty VPC ID is an error",
			region:              "us-west-1",
			teleportClusterName: "foo.bar.com",
			vpcID:               "",
			wantErrContains:     "empty VPC",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ECSDatabaseServiceDashboardURL(tt.region, tt.teleportClusterName, tt.vpcID)
			if tt.wantErrContains != "" {
				require.Error(t, err)
				require.ErrorContains(t, err, tt.wantErrContains)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantURL, got)
		})
	}
}
