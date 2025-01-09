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

package awsoidc

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestListDeployedDatabaseServicesRequest(t *testing.T) {
	isBadParamErrFn := func(tt require.TestingT, err error, i ...any) {
		require.True(tt, trace.IsBadParameter(err), "expected bad parameter, got %v", err)
	}

	baseReqFn := func() ListDeployedDatabaseServicesRequest {
		return ListDeployedDatabaseServicesRequest{
			TeleportClusterName: "mycluster",
			Region:              "eu-west-2",
			Integration:         "my-integration",
		}
	}

	for _, tt := range []struct {
		name            string
		req             func() ListDeployedDatabaseServicesRequest
		errCheck        require.ErrorAssertionFunc
		reqWithDefaults ListDeployedDatabaseServicesRequest
	}{
		{
			name: "no fields",
			req: func() ListDeployedDatabaseServicesRequest {
				return ListDeployedDatabaseServicesRequest{}
			},
			errCheck: isBadParamErrFn,
		},
		{
			name: "missing teleport cluster name",
			req: func() ListDeployedDatabaseServicesRequest {
				r := baseReqFn()
				r.TeleportClusterName = ""
				return r
			},
			errCheck: isBadParamErrFn,
		},
		{
			name: "missing region",
			req: func() ListDeployedDatabaseServicesRequest {
				r := baseReqFn()
				r.Region = ""
				return r
			},
			errCheck: isBadParamErrFn,
		},
		{
			name: "missing integration",
			req: func() ListDeployedDatabaseServicesRequest {
				r := baseReqFn()
				r.Integration = ""
				return r
			},
			errCheck: isBadParamErrFn,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := tt.req()
			err := r.checkAndSetDefaults()
			tt.errCheck(t, err)

			if err != nil {
				return
			}

			require.Empty(t, cmp.Diff(tt.reqWithDefaults, r))
		})
	}
}

type mockListECSClient struct {
	pageSize int

	clusterName    string
	services       []*ecstypes.Service
	mapServices    map[string]ecstypes.Service
	taskDefinition map[string]*ecstypes.TaskDefinition
}

func (m *mockListECSClient) ListServices(ctx context.Context, params *ecs.ListServicesInput, optFns ...func(*ecs.Options)) (*ecs.ListServicesOutput, error) {
	if aws.ToString(params.Cluster) != m.clusterName || len(m.services) == 0 {
		return nil, trace.NotFound("ECS Cluster not found")
	}

	ret := &ecs.ListServicesOutput{}
	requestedPage := 1

	totalEndpoints := len(m.services)

	if params.NextToken != nil {
		currentMarker, err := strconv.Atoi(*params.NextToken)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		requestedPage = currentMarker
	}

	sliceStart := m.pageSize * (requestedPage - 1)
	sliceEnd := m.pageSize * requestedPage
	if sliceEnd > totalEndpoints {
		sliceEnd = totalEndpoints
	}

	for _, service := range m.services[sliceStart:sliceEnd] {
		ret.ServiceArns = append(ret.ServiceArns, aws.ToString(service.ServiceArn))
	}

	if sliceEnd < totalEndpoints {
		nextToken := strconv.Itoa(requestedPage + 1)
		ret.NextToken = &nextToken
	}

	return ret, nil
}

func (m *mockListECSClient) DescribeServices(ctx context.Context, params *ecs.DescribeServicesInput, optFns ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error) {
	ret := &ecs.DescribeServicesOutput{}
	if aws.ToString(params.Cluster) != m.clusterName {
		return ret, nil
	}

	for _, serviceARN := range params.Services {
		ret.Services = append(ret.Services, m.mapServices[serviceARN])
	}
	return ret, nil
}

func (m *mockListECSClient) DescribeTaskDefinition(ctx context.Context, params *ecs.DescribeTaskDefinitionInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTaskDefinitionOutput, error) {
	ret := &ecs.DescribeTaskDefinitionOutput{}
	ret.TaskDefinition = m.taskDefinition[aws.ToString(params.TaskDefinition)]

	return ret, nil
}

func dummyServiceTask(idx int) (ecstypes.Service, *ecstypes.TaskDefinition) {
	taskName := fmt.Sprintf("task-family-name-%d", idx)
	serviceARN := fmt.Sprintf("arn:eks:service-%d", idx)

	ecsTask := &ecstypes.TaskDefinition{
		Family: aws.String(taskName),
		ContainerDefinitions: []ecstypes.ContainerDefinition{{
			EntryPoint: []string{"teleport"},
			Command:    []string{"start"},
		}},
	}

	ecsService := ecstypes.Service{
		ServiceArn:     aws.String(serviceARN),
		ServiceName:    aws.String(fmt.Sprintf("database-service-vpc-%d", idx)),
		TaskDefinition: aws.String(taskName),
		Tags: []ecstypes.Tag{
			{Key: aws.String("teleport.dev/cluster"), Value: aws.String("my-cluster")},
			{Key: aws.String("teleport.dev/integration"), Value: aws.String("my-integration")},
			{Key: aws.String("teleport.dev/origin"), Value: aws.String("integration_awsoidc")},
		},
	}

	return ecsService, ecsTask
}

func TestListDeployedDatabaseServices(t *testing.T) {
	ctx := context.Background()

	const pageSize = 100
	t.Run("pagination", func(t *testing.T) {
		totalServices := 203

		allServices := make([]*ecstypes.Service, 0, totalServices)
		mapServices := make(map[string]ecstypes.Service, totalServices)
		allTasks := make(map[string]*ecstypes.TaskDefinition, totalServices)
		for i := 0; i < totalServices; i++ {
			ecsService, ecsTask := dummyServiceTask(i)
			allTasks[aws.ToString(ecsTask.Family)] = ecsTask
			mapServices[aws.ToString(ecsService.ServiceArn)] = ecsService
			allServices = append(allServices, &ecsService)
		}

		mockListClient := &mockListECSClient{
			pageSize:       pageSize,
			clusterName:    "my-cluster-teleport",
			mapServices:    mapServices,
			services:       allServices,
			taskDefinition: allTasks,
		}

		// First page must return pageSize number of Endpoints
		resp, err := ListDeployedDatabaseServices(ctx, mockListClient, ListDeployedDatabaseServicesRequest{
			Integration:         "my-integration",
			TeleportClusterName: "my-cluster",
			Region:              "us-east-1",
		})
		require.NoError(t, err)
		require.NotEmpty(t, resp.NextToken)
		require.Len(t, resp.DeployedDatabaseServices, pageSize)
		require.Equal(t, "database-service-vpc-0", resp.DeployedDatabaseServices[0].Name)
		require.Equal(t, "https://us-east-1.console.aws.amazon.com/ecs/v2/clusters/my-cluster-teleport/services/database-service-vpc-0", resp.DeployedDatabaseServices[0].ServiceDashboardURL)
		require.Equal(t, []string{"teleport"}, resp.DeployedDatabaseServices[0].ContainerEntryPoint)
		require.Equal(t, []string{"start"}, resp.DeployedDatabaseServices[0].ContainerCommand)

		// Second page must return pageSize number of Endpoints
		nextPageToken := resp.NextToken
		resp, err = ListDeployedDatabaseServices(ctx, mockListClient, ListDeployedDatabaseServicesRequest{
			Integration:         "my-integration",
			TeleportClusterName: "my-cluster",
			Region:              "us-east-1",
			NextToken:           nextPageToken,
		})
		require.NoError(t, err)
		require.NotEmpty(t, resp.NextToken)
		require.Len(t, resp.DeployedDatabaseServices, pageSize)
		require.Equal(t, "https://us-east-1.console.aws.amazon.com/ecs/v2/clusters/my-cluster-teleport/services/database-service-vpc-100", resp.DeployedDatabaseServices[0].ServiceDashboardURL)

		// Third page must return only the remaining Endpoints and an empty nextToken
		nextPageToken = resp.NextToken
		resp, err = ListDeployedDatabaseServices(ctx, mockListClient, ListDeployedDatabaseServicesRequest{
			Integration:         "my-integration",
			TeleportClusterName: "my-cluster",
			Region:              "us-east-1",
			NextToken:           nextPageToken,
		})
		require.NoError(t, err)
		require.Empty(t, resp.NextToken)
		require.Len(t, resp.DeployedDatabaseServices, 3)
	})

	for _, tt := range []struct {
		name       string
		req        ListDeployedDatabaseServicesRequest
		mockClient func() *mockListECSClient
		errCheck   require.ErrorAssertionFunc
		respCheck  func(*testing.T, *ListDeployedDatabaseServicesResponse)
	}{
		{
			name: "ignores ECS Services without ownership tags",
			req: ListDeployedDatabaseServicesRequest{
				Integration:         "my-integration",
				TeleportClusterName: "my-cluster",
				Region:              "us-east-1",
			},
			mockClient: func() *mockListECSClient {
				ret := &mockListECSClient{
					pageSize:    10,
					clusterName: "my-cluster-teleport",
				}
				ecsService, ecsTask := dummyServiceTask(0)

				ecsServiceAnotherIntegration, ecsTaskAnotherIntegration := dummyServiceTask(1)
				ecsServiceAnotherIntegration.Tags = []ecstypes.Tag{{Key: aws.String("teleport.dev/integration"), Value: aws.String("another-integration")}}

				ret.taskDefinition = map[string]*ecstypes.TaskDefinition{
					aws.ToString(ecsTask.Family):                   ecsTask,
					aws.ToString(ecsTaskAnotherIntegration.Family): ecsTaskAnotherIntegration,
				}
				ret.mapServices = map[string]ecstypes.Service{
					aws.ToString(ecsService.ServiceArn):                   ecsService,
					aws.ToString(ecsServiceAnotherIntegration.ServiceArn): ecsServiceAnotherIntegration,
				}
				ret.services = append(ret.services, &ecsService)
				ret.services = append(ret.services, &ecsServiceAnotherIntegration)
				return ret
			},
			respCheck: func(t *testing.T, resp *ListDeployedDatabaseServicesResponse) {
				require.Len(t, resp.DeployedDatabaseServices, 1, "expected 1 service, got %d", len(resp.DeployedDatabaseServices))
				require.Empty(t, resp.NextToken, "expected an empty NextToken")

				expectedService := DeployedDatabaseService{
					Name:                "database-service-vpc-0",
					ServiceDashboardURL: "https://us-east-1.console.aws.amazon.com/ecs/v2/clusters/my-cluster-teleport/services/database-service-vpc-0",
					ContainerEntryPoint: []string{"teleport"},
					ContainerCommand:    []string{"start"},
				}
				require.Empty(t, cmp.Diff(expectedService, resp.DeployedDatabaseServices[0]))
			},
			errCheck: require.NoError,
		},
		{
			name: "ignores ECS Services without containers",
			req: ListDeployedDatabaseServicesRequest{
				Integration:         "my-integration",
				TeleportClusterName: "my-cluster",
				Region:              "us-east-1",
			},
			mockClient: func() *mockListECSClient {
				ret := &mockListECSClient{
					pageSize:    10,
					clusterName: "my-cluster-teleport",
				}
				ecsService, ecsTask := dummyServiceTask(0)

				ecsServiceWithoutContainers, ecsTaskWithoutContainers := dummyServiceTask(1)
				ecsTaskWithoutContainers.ContainerDefinitions = []ecstypes.ContainerDefinition{}

				ret.taskDefinition = map[string]*ecstypes.TaskDefinition{
					aws.ToString(ecsTask.Family):                  ecsTask,
					aws.ToString(ecsTaskWithoutContainers.Family): ecsTaskWithoutContainers,
				}
				ret.mapServices = map[string]ecstypes.Service{
					aws.ToString(ecsService.ServiceArn):                  ecsService,
					aws.ToString(ecsServiceWithoutContainers.ServiceArn): ecsServiceWithoutContainers,
				}
				ret.services = append(ret.services, &ecsService)
				ret.services = append(ret.services, &ecsServiceWithoutContainers)
				return ret
			},
			respCheck: func(t *testing.T, resp *ListDeployedDatabaseServicesResponse) {
				require.Len(t, resp.DeployedDatabaseServices, 1, "expected 1 service, got %d", len(resp.DeployedDatabaseServices))
				require.Empty(t, resp.NextToken, "expected an empty NextToken")

				expectedService := DeployedDatabaseService{
					Name:                "database-service-vpc-0",
					ServiceDashboardURL: "https://us-east-1.console.aws.amazon.com/ecs/v2/clusters/my-cluster-teleport/services/database-service-vpc-0",
					ContainerEntryPoint: []string{"teleport"},
					ContainerCommand:    []string{"start"},
				}
				require.Empty(t, cmp.Diff(expectedService, resp.DeployedDatabaseServices[0]))
			},
			errCheck: require.NoError,
		},
		{
			name: "returns empty list when the ECS Cluster does not exist",
			req: ListDeployedDatabaseServicesRequest{
				Integration:         "my-integration",
				TeleportClusterName: "my-cluster",
				Region:              "us-east-1",
			},
			mockClient: func() *mockListECSClient {
				ret := &mockListECSClient{
					pageSize: 10,
				}
				return ret
			},
			respCheck: func(t *testing.T, resp *ListDeployedDatabaseServicesResponse) {
				require.Empty(t, resp.DeployedDatabaseServices, "expected 0 services")
				require.Empty(t, resp.NextToken, "expected an empty NextToken")
			},
			errCheck: require.NoError,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := ListDeployedDatabaseServices(ctx, tt.mockClient(), tt.req)
			tt.errCheck(t, err)
			if tt.respCheck != nil {
				tt.respCheck(t, resp)
			}
		})
	}
}
