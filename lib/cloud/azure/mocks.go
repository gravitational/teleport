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

package azure

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/msi/armmsi"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/mysql/armmysql"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/mysql/armmysqlflexibleservers"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresql"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresqlflexibleservers"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/redis/armredis/v3"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/redisenterprise/armredisenterprise"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/sql/armsql"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/subscription/armsubscription"
	"github.com/gravitational/trace"
)

type ARMSubscriptionsMock struct {
	Subscriptions []*armsubscription.Subscription
	NoAuth        bool
}

var _ ARMSubscriptions = (*ARMSubscriptionsMock)(nil)

func (m *ARMSubscriptionsMock) NewListPager(_ *armsubscription.SubscriptionsClientListOptions) *runtime.Pager[armsubscription.SubscriptionsClientListResponse] {
	return runtime.NewPager(runtime.PagingHandler[armsubscription.SubscriptionsClientListResponse]{
		More: func(page armsubscription.SubscriptionsClientListResponse) bool {
			return page.NextLink != nil && len(*page.NextLink) > 0
		},
		Fetcher: func(ctx context.Context, page *armsubscription.SubscriptionsClientListResponse) (armsubscription.SubscriptionsClientListResponse, error) {
			if m.NoAuth {
				return armsubscription.SubscriptionsClientListResponse{}, trace.AccessDenied("unauthorized")
			}
			return armsubscription.SubscriptionsClientListResponse{
				ListResult: armsubscription.ListResult{
					Value: m.Subscriptions,
				},
			}, nil
		},
	})
}

// ARMMySQLMock mocks Azure armmysql API.
type ARMMySQLMock struct {
	DBServers []*armmysql.Server
	NoAuth    bool
}

var _ ARMMySQL = (*ARMMySQLMock)(nil)

func (m *ARMMySQLMock) Get(_ context.Context, group, name string, _ *armmysql.ServersClientGetOptions) (armmysql.ServersClientGetResponse, error) {
	if m.NoAuth {
		return armmysql.ServersClientGetResponse{}, trace.AccessDenied("unauthorized")
	}
	for _, s := range m.DBServers {
		if name == *s.Name {
			id, err := arm.ParseResourceID(*s.ID)
			if err != nil {
				return armmysql.ServersClientGetResponse{}, trace.Wrap(err)
			}
			if group == id.ResourceGroupName {
				return armmysql.ServersClientGetResponse{Server: *s}, nil
			}
		}
	}
	return armmysql.ServersClientGetResponse{}, trace.NotFound("resource %v in group %v not found", name, group)
}

func (m *ARMMySQLMock) NewListPager(_ *armmysql.ServersClientListOptions) *runtime.Pager[armmysql.ServersClientListResponse] {
	return runtime.NewPager(runtime.PagingHandler[armmysql.ServersClientListResponse]{
		More: func(_ armmysql.ServersClientListResponse) bool {
			return false
		},
		Fetcher: func(_ context.Context, _ *armmysql.ServersClientListResponse) (armmysql.ServersClientListResponse, error) {
			if m.NoAuth {
				return armmysql.ServersClientListResponse{}, trace.AccessDenied("unauthorized")
			}
			return armmysql.ServersClientListResponse{
				ServerListResult: armmysql.ServerListResult{
					Value: m.DBServers,
				},
			}, nil
		},
	})
}

func (m *ARMMySQLMock) NewListByResourceGroupPager(group string, _ *armmysql.ServersClientListByResourceGroupOptions) *runtime.Pager[armmysql.ServersClientListByResourceGroupResponse] {
	return runtime.NewPager(runtime.PagingHandler[armmysql.ServersClientListByResourceGroupResponse]{
		More: func(_ armmysql.ServersClientListByResourceGroupResponse) bool {
			return false
		},
		Fetcher: func(_ context.Context, _ *armmysql.ServersClientListByResourceGroupResponse) (armmysql.ServersClientListByResourceGroupResponse, error) {
			if m.NoAuth {
				return armmysql.ServersClientListByResourceGroupResponse{}, trace.AccessDenied("unauthorized")
			}
			var servers []*armmysql.Server
			for _, s := range m.DBServers {
				id, err := arm.ParseResourceID(*s.ID)
				if err != nil {
					return armmysql.ServersClientListByResourceGroupResponse{}, trace.Wrap(err)
				}
				if group == id.ResourceGroupName {
					servers = append(servers, s)
				}
			}
			if len(servers) == 0 {
				return armmysql.ServersClientListByResourceGroupResponse{}, trace.NotFound("Resource group '%v' could not be found.", group)
			}
			return armmysql.ServersClientListByResourceGroupResponse{
				ServerListResult: armmysql.ServerListResult{
					Value: servers,
				},
			}, nil
		},
	})
}

// ARMPostgresMock mocks Azure armpostgresql API.
type ARMPostgresMock struct {
	DBServers []*armpostgresql.Server
	NoAuth    bool
}

var _ ARMPostgres = (*ARMPostgresMock)(nil)

func (m *ARMPostgresMock) Get(_ context.Context, group, name string, _ *armpostgresql.ServersClientGetOptions) (armpostgresql.ServersClientGetResponse, error) {
	if m.NoAuth {
		return armpostgresql.ServersClientGetResponse{}, trace.AccessDenied("unauthorized")
	}
	for _, s := range m.DBServers {
		if name == *s.Name {
			id, err := arm.ParseResourceID(*s.ID)
			if err != nil {
				return armpostgresql.ServersClientGetResponse{}, trace.Wrap(err)
			}
			if group == id.ResourceGroupName {
				return armpostgresql.ServersClientGetResponse{Server: *s}, nil
			}
		}
	}
	return armpostgresql.ServersClientGetResponse{}, trace.NotFound("resource %v in group %v not found", name, group)
}

func (m *ARMPostgresMock) NewListPager(_ *armpostgresql.ServersClientListOptions) *runtime.Pager[armpostgresql.ServersClientListResponse] {
	return runtime.NewPager(runtime.PagingHandler[armpostgresql.ServersClientListResponse]{
		More: func(_ armpostgresql.ServersClientListResponse) bool {
			return false
		},
		Fetcher: func(_ context.Context, _ *armpostgresql.ServersClientListResponse) (armpostgresql.ServersClientListResponse, error) {
			if m.NoAuth {
				return armpostgresql.ServersClientListResponse{}, trace.AccessDenied("unauthorized")
			}
			return armpostgresql.ServersClientListResponse{
				ServerListResult: armpostgresql.ServerListResult{
					Value: m.DBServers,
				},
			}, nil
		},
	})
}

func (m *ARMPostgresMock) NewListByResourceGroupPager(group string, _ *armpostgresql.ServersClientListByResourceGroupOptions) *runtime.Pager[armpostgresql.ServersClientListByResourceGroupResponse] {
	return runtime.NewPager(runtime.PagingHandler[armpostgresql.ServersClientListByResourceGroupResponse]{
		More: func(_ armpostgresql.ServersClientListByResourceGroupResponse) bool {
			return false
		},
		Fetcher: func(_ context.Context, _ *armpostgresql.ServersClientListByResourceGroupResponse) (armpostgresql.ServersClientListByResourceGroupResponse, error) {
			if m.NoAuth {
				return armpostgresql.ServersClientListByResourceGroupResponse{}, trace.AccessDenied("unauthorized")
			}
			var servers []*armpostgresql.Server
			for _, s := range m.DBServers {
				id, err := arm.ParseResourceID(*s.ID)
				if err != nil {
					return armpostgresql.ServersClientListByResourceGroupResponse{}, trace.Wrap(err)
				}
				if group == id.ResourceGroupName {
					servers = append(servers, s)
				}
			}
			if len(servers) == 0 {
				return armpostgresql.ServersClientListByResourceGroupResponse{}, trace.NotFound("Resource group '%v' could not be found.", group)
			}
			return armpostgresql.ServersClientListByResourceGroupResponse{
				ServerListResult: armpostgresql.ServerListResult{
					Value: servers,
				},
			}, nil
		},
	})
}

// ARMRedisMock mocks armRedisClient.
type ARMRedisMock struct {
	Token   string
	NoAuth  bool
	Servers []*armredis.ResourceInfo
}

func (m *ARMRedisMock) ListKeys(ctx context.Context, resourceGroupName string, name string, options *armredis.ClientListKeysOptions) (armredis.ClientListKeysResponse, error) {
	if m.NoAuth {
		return armredis.ClientListKeysResponse{}, trace.AccessDenied("unauthorized")
	}
	return armredis.ClientListKeysResponse{
		AccessKeys: armredis.AccessKeys{
			PrimaryKey: &m.Token,
		},
	}, nil
}

func (m *ARMRedisMock) NewListBySubscriptionPager(options *armredis.ClientListBySubscriptionOptions) *runtime.Pager[armredis.ClientListBySubscriptionResponse] {
	return newPagerHelper(m.NoAuth, func() (armredis.ClientListBySubscriptionResponse, error) {
		return armredis.ClientListBySubscriptionResponse{
			ListResult: armredis.ListResult{
				Value: m.Servers,
			},
		}, nil
	})
}
func (m *ARMRedisMock) NewListByResourceGroupPager(resourceGroupName string, options *armredis.ClientListByResourceGroupOptions) *runtime.Pager[armredis.ClientListByResourceGroupResponse] {
	return newPagerHelper(m.NoAuth, func() (armredis.ClientListByResourceGroupResponse, error) {
		var servers []*armredis.ResourceInfo
		for _, server := range m.Servers {
			id, err := arm.ParseResourceID(StringVal(server.ID))
			if err != nil {
				return armredis.ClientListByResourceGroupResponse{}, trace.Wrap(err)
			}
			if resourceGroupName == id.ResourceGroupName {
				servers = append(servers, server)
			}
		}
		if len(servers) == 0 {
			return armredis.ClientListByResourceGroupResponse{}, trace.NotFound("no resources found")
		}
		return armredis.ClientListByResourceGroupResponse{
			ListResult: armredis.ListResult{
				Value: servers,
			},
		}, nil
	})
}

// ARMRedisEnterpriseDatabaseMock mocks armRedisEnterpriseDatabaseClient.
type ARMRedisEnterpriseDatabaseMock struct {
	Token                string
	TokensByDatabaseName map[string]string
	NoAuth               bool
	Databases            []*armredisenterprise.Database
}

func (m *ARMRedisEnterpriseDatabaseMock) ListKeys(ctx context.Context, resourceGroupName string, clusterName string, databaseName string, options *armredisenterprise.DatabasesClientListKeysOptions) (armredisenterprise.DatabasesClientListKeysResponse, error) {
	if m.NoAuth {
		return armredisenterprise.DatabasesClientListKeysResponse{}, trace.AccessDenied("unauthorized")
	}
	if len(m.TokensByDatabaseName) != 0 {
		if token, found := m.TokensByDatabaseName[databaseName]; found {
			return armredisenterprise.DatabasesClientListKeysResponse{
				AccessKeys: armredisenterprise.AccessKeys{
					PrimaryKey: &token,
				},
			}, nil
		}
	}
	return armredisenterprise.DatabasesClientListKeysResponse{
		AccessKeys: armredisenterprise.AccessKeys{
			PrimaryKey: &m.Token,
		},
	}, nil
}

func (m *ARMRedisEnterpriseDatabaseMock) NewListByClusterPager(resourceGroupName string, clusterName string, options *armredisenterprise.DatabasesClientListByClusterOptions) *runtime.Pager[armredisenterprise.DatabasesClientListByClusterResponse] {
	return newPagerHelper(m.NoAuth, func() (armredisenterprise.DatabasesClientListByClusterResponse, error) {
		var databases []*armredisenterprise.Database
		for _, database := range m.Databases {
			id, err := arm.ParseResourceID(StringVal(database.ID))
			if err != nil {
				return armredisenterprise.DatabasesClientListByClusterResponse{}, trace.Wrap(err)
			}
			if resourceGroupName == id.ResourceGroupName && id.Parent != nil && id.Parent.Name == clusterName {
				databases = append(databases, database)
			}
		}
		if len(databases) == 0 {
			return armredisenterprise.DatabasesClientListByClusterResponse{}, trace.NotFound("no resources found")
		}
		return armredisenterprise.DatabasesClientListByClusterResponse{
			DatabaseList: armredisenterprise.DatabaseList{
				Value: databases,
			},
		}, nil
	})
}

// ARMRedisEnterpriseClusterMock mocks armRedisEnterpriseClusterClient.
type ARMRedisEnterpriseClusterMock struct {
	NoAuth   bool
	Clusters []*armredisenterprise.Cluster
}

func (m *ARMRedisEnterpriseClusterMock) NewListPager(options *armredisenterprise.ClientListOptions) *runtime.Pager[armredisenterprise.ClientListResponse] {
	return newPagerHelper(m.NoAuth, func() (armredisenterprise.ClientListResponse, error) {
		return armredisenterprise.ClientListResponse{
			ClusterList: armredisenterprise.ClusterList{
				Value: m.Clusters,
			},
		}, nil
	})
}
func (m *ARMRedisEnterpriseClusterMock) NewListByResourceGroupPager(resourceGroupName string, options *armredisenterprise.ClientListByResourceGroupOptions) *runtime.Pager[armredisenterprise.ClientListByResourceGroupResponse] {
	return newPagerHelper(m.NoAuth, func() (armredisenterprise.ClientListByResourceGroupResponse, error) {
		var clusters []*armredisenterprise.Cluster
		for _, cluster := range m.Clusters {
			id, err := arm.ParseResourceID(StringVal(cluster.ID))
			if err != nil {
				return armredisenterprise.ClientListByResourceGroupResponse{}, trace.Wrap(err)
			}
			if resourceGroupName == id.ResourceGroupName {
				clusters = append(clusters, cluster)
			}
		}
		if len(clusters) == 0 {
			return armredisenterprise.ClientListByResourceGroupResponse{}, trace.NotFound("no resources found")
		}
		return armredisenterprise.ClientListByResourceGroupResponse{
			ClusterList: armredisenterprise.ClusterList{
				Value: clusters,
			},
		}, nil
	})
}

// newPagerHelper is a helper for creating a runtime.Pager for common ARM mocks.
func newPagerHelper[T any](noAuth bool, newT func() (T, error)) *runtime.Pager[T] {
	return runtime.NewPager(runtime.PagingHandler[T]{
		More: func(_ T) bool {
			return false
		},
		Fetcher: func(_ context.Context, _ *T) (T, error) {
			if noAuth {
				var t T
				return t, trace.AccessDenied("unauthorized")
			}
			return newT()
		},
	})
}

// ARMKubernetesMock mocks Azure armmanagedclusters API.
type ARMKubernetesMock struct {
	KubeServers       []*armcontainerservice.ManagedCluster
	ClusterAdminCreds *armcontainerservice.CredentialResult
	ClusterUserCreds  *armcontainerservice.CredentialResult
	NoAuth            bool
}

var _ ARMAKS = (*ARMKubernetesMock)(nil)

func (m *ARMKubernetesMock) Get(_ context.Context, group, name string, _ *armcontainerservice.ManagedClustersClientGetOptions) (armcontainerservice.ManagedClustersClientGetResponse, error) {
	if m.NoAuth {
		return armcontainerservice.ManagedClustersClientGetResponse{}, trace.AccessDenied("unauthorized")
	}
	for _, s := range m.KubeServers {
		if name == *s.Name {
			id, err := arm.ParseResourceID(*s.ID)
			if err != nil {
				return armcontainerservice.ManagedClustersClientGetResponse{}, trace.Wrap(err)
			}
			if group == id.ResourceGroupName {
				return armcontainerservice.ManagedClustersClientGetResponse{ManagedCluster: *s}, nil
			}
		}
	}
	return armcontainerservice.ManagedClustersClientGetResponse{}, trace.NotFound("resource %v in group %v not found", name, group)
}

func (m *ARMKubernetesMock) NewListPager(_ *armcontainerservice.ManagedClustersClientListOptions) *runtime.Pager[armcontainerservice.ManagedClustersClientListResponse] {
	return runtime.NewPager(runtime.PagingHandler[armcontainerservice.ManagedClustersClientListResponse]{
		More: func(_ armcontainerservice.ManagedClustersClientListResponse) bool {
			return false
		},
		Fetcher: func(_ context.Context, _ *armcontainerservice.ManagedClustersClientListResponse) (armcontainerservice.ManagedClustersClientListResponse, error) {
			if m.NoAuth {
				return armcontainerservice.ManagedClustersClientListResponse{}, trace.AccessDenied("unauthorized")
			}
			return armcontainerservice.ManagedClustersClientListResponse{
				ManagedClusterListResult: armcontainerservice.ManagedClusterListResult{
					Value: m.KubeServers,
				},
			}, nil
		},
	})
}

func (m *ARMKubernetesMock) NewListByResourceGroupPager(group string, _ *armcontainerservice.ManagedClustersClientListByResourceGroupOptions) *runtime.Pager[armcontainerservice.ManagedClustersClientListByResourceGroupResponse] {
	return runtime.NewPager(runtime.PagingHandler[armcontainerservice.ManagedClustersClientListByResourceGroupResponse]{
		More: func(_ armcontainerservice.ManagedClustersClientListByResourceGroupResponse) bool {
			return false
		},
		Fetcher: func(_ context.Context, _ *armcontainerservice.ManagedClustersClientListByResourceGroupResponse) (armcontainerservice.ManagedClustersClientListByResourceGroupResponse, error) {
			if m.NoAuth {
				return armcontainerservice.ManagedClustersClientListByResourceGroupResponse{}, trace.AccessDenied("unauthorized")
			}
			var servers []*armcontainerservice.ManagedCluster
			for _, s := range m.KubeServers {
				id, err := arm.ParseResourceID(*s.ID)
				if err != nil {
					return armcontainerservice.ManagedClustersClientListByResourceGroupResponse{}, trace.Wrap(err)
				}
				if group == id.ResourceGroupName {
					servers = append(servers, s)
				}
			}
			if len(servers) == 0 {
				return armcontainerservice.ManagedClustersClientListByResourceGroupResponse{}, trace.NotFound("Resource group '%v' could not be found.", group)
			}
			return armcontainerservice.ManagedClustersClientListByResourceGroupResponse{
				ManagedClusterListResult: armcontainerservice.ManagedClusterListResult{
					Value: servers,
				},
			}, nil
		},
	})
}

func (m *ARMKubernetesMock) GetCommandResult(ctx context.Context, resourceGroupName string, resourceName string, commandID string, options *armcontainerservice.ManagedClustersClientGetCommandResultOptions) (armcontainerservice.ManagedClustersClientGetCommandResultResponse, error) {
	return armcontainerservice.ManagedClustersClientGetCommandResultResponse{
		RunCommandResult: armcontainerservice.RunCommandResult{
			ID: to.Ptr(commandID),
		},
	}, nil
}
func (m *ARMKubernetesMock) ListClusterAdminCredentials(ctx context.Context, resourceGroupName string, resourceName string, options *armcontainerservice.ManagedClustersClientListClusterAdminCredentialsOptions) (armcontainerservice.ManagedClustersClientListClusterAdminCredentialsResponse, error) {
	if m.NoAuth {
		return armcontainerservice.ManagedClustersClientListClusterAdminCredentialsResponse{}, trace.AccessDenied("unauthorized")
	}

	return armcontainerservice.ManagedClustersClientListClusterAdminCredentialsResponse{
		CredentialResults: armcontainerservice.CredentialResults{
			Kubeconfigs: []*armcontainerservice.CredentialResult{
				m.ClusterAdminCreds,
			},
		},
	}, nil
}
func (m *ARMKubernetesMock) ListClusterUserCredentials(ctx context.Context, resourceGroupName string, resourceName string, options *armcontainerservice.ManagedClustersClientListClusterUserCredentialsOptions) (armcontainerservice.ManagedClustersClientListClusterUserCredentialsResponse, error) {
	if m.NoAuth {
		return armcontainerservice.ManagedClustersClientListClusterUserCredentialsResponse{}, trace.AccessDenied("unauthorized")
	}
	return armcontainerservice.ManagedClustersClientListClusterUserCredentialsResponse{
		CredentialResults: armcontainerservice.CredentialResults{
			Kubeconfigs: []*armcontainerservice.CredentialResult{
				m.ClusterUserCreds,
			},
		},
	}, nil
}

func (m *ARMKubernetesMock) BeginRunCommand(ctx context.Context, resourceGroupName string, resourceName string, requestPayload armcontainerservice.RunCommandRequest, options *armcontainerservice.ManagedClustersClientBeginRunCommandOptions) (*runtime.Poller[armcontainerservice.ManagedClustersClientRunCommandResponse], error) {
	if m.NoAuth {
		return nil, trace.AccessDenied("unauthorized")
	}
	return &runtime.Poller[armcontainerservice.ManagedClustersClientRunCommandResponse]{}, nil
}

// ARMComputeMock mocks armcompute.VirtualMachinesClient.
type ARMComputeMock struct {
	VirtualMachines map[string][]*armcompute.VirtualMachine
	GetResult       armcompute.VirtualMachine
	GetErr          error
}

func (m *ARMComputeMock) NewListPager(resourceGroup string, _ *armcompute.VirtualMachinesClientListOptions) *runtime.Pager[armcompute.VirtualMachinesClientListResponse] {
	vms, ok := m.VirtualMachines[resourceGroup]
	if !ok {
		vms = []*armcompute.VirtualMachine{}
	}
	return runtime.NewPager(runtime.PagingHandler[armcompute.VirtualMachinesClientListResponse]{
		More: func(page armcompute.VirtualMachinesClientListResponse) bool {
			return page.NextLink != nil && len(*page.NextLink) > 0
		},
		Fetcher: func(ctx context.Context, page *armcompute.VirtualMachinesClientListResponse) (armcompute.VirtualMachinesClientListResponse, error) {
			return armcompute.VirtualMachinesClientListResponse{
				VirtualMachineListResult: armcompute.VirtualMachineListResult{
					Value: vms,
				},
			}, nil
		},
	})
}

func (m *ARMComputeMock) NewListAllPager(_ *armcompute.VirtualMachinesClientListAllOptions) *runtime.Pager[armcompute.VirtualMachinesClientListAllResponse] {
	var vms []*armcompute.VirtualMachine
	for _, resourceGroupVMs := range m.VirtualMachines {
		vms = append(vms, resourceGroupVMs...)
	}
	return runtime.NewPager(runtime.PagingHandler[armcompute.VirtualMachinesClientListAllResponse]{
		More: func(page armcompute.VirtualMachinesClientListAllResponse) bool {
			return page.NextLink != nil && len(*page.NextLink) > 0
		},
		Fetcher: func(ctx context.Context, page *armcompute.VirtualMachinesClientListAllResponse) (armcompute.VirtualMachinesClientListAllResponse, error) {
			return armcompute.VirtualMachinesClientListAllResponse{
				VirtualMachineListResult: armcompute.VirtualMachineListResult{
					Value: vms,
				},
			}, nil
		},
	})
}

func (m *ARMComputeMock) Get(_ context.Context, _ string, _ string, _ *armcompute.VirtualMachinesClientGetOptions) (armcompute.VirtualMachinesClientGetResponse, error) {
	return armcompute.VirtualMachinesClientGetResponse{
		VirtualMachine: m.GetResult,
	}, m.GetErr
}

// ARMComputeScaleSetMock mocks armcompute.VirtualMachineScaleSetVMsClient.
type ARMScaleSetMock struct {
	GetResult armcompute.VirtualMachineScaleSetVM
	GetErr    error
}

func (m *ARMScaleSetMock) Get(ctx context.Context, resourceGroupName string, vmScaleSetName string, instanceID string, options *armcompute.VirtualMachineScaleSetVMsClientGetOptions) (armcompute.VirtualMachineScaleSetVMsClientGetResponse, error) {
	return armcompute.VirtualMachineScaleSetVMsClientGetResponse{
		VirtualMachineScaleSetVM: m.GetResult,
	}, m.GetErr
}

// ARMSQLServerMock mocks armSQLServerClient
type ARMSQLServerMock struct {
	NoAuth               bool
	AllServers           []*armsql.Server
	ResourceGroupServers []*armsql.Server
}

func (m *ARMSQLServerMock) NewListPager(options *armsql.ServersClientListOptions) *runtime.Pager[armsql.ServersClientListResponse] {
	return newPagerHelper(m.NoAuth, func() (armsql.ServersClientListResponse, error) {
		return armsql.ServersClientListResponse{
			ServerListResult: armsql.ServerListResult{
				Value: m.AllServers,
			},
		}, nil
	})
}

func (m *ARMSQLServerMock) NewListByResourceGroupPager(resourceGroupName string, options *armsql.ServersClientListByResourceGroupOptions) *runtime.Pager[armsql.ServersClientListByResourceGroupResponse] {
	return newPagerHelper(m.NoAuth, func() (armsql.ServersClientListByResourceGroupResponse, error) {
		return armsql.ServersClientListByResourceGroupResponse{
			ServerListResult: armsql.ServerListResult{
				Value: m.ResourceGroupServers,
			},
		}, nil
	})
}

// ARMSQLManagedServerMock mocks armSQLServerClient
type ARMSQLManagedServerMock struct {
	NoAuth               bool
	AllServers           []*armsql.ManagedInstance
	ResourceGroupServers []*armsql.ManagedInstance
}

func (m *ARMSQLManagedServerMock) NewListPager(options *armsql.ManagedInstancesClientListOptions) *runtime.Pager[armsql.ManagedInstancesClientListResponse] {
	return newPagerHelper(m.NoAuth, func() (armsql.ManagedInstancesClientListResponse, error) {
		return armsql.ManagedInstancesClientListResponse{
			ManagedInstanceListResult: armsql.ManagedInstanceListResult{
				Value: m.AllServers,
			},
		}, nil
	})
}

func (m *ARMSQLManagedServerMock) NewListByResourceGroupPager(resourceGroupName string, options *armsql.ManagedInstancesClientListByResourceGroupOptions) *runtime.Pager[armsql.ManagedInstancesClientListByResourceGroupResponse] {
	return newPagerHelper(m.NoAuth, func() (armsql.ManagedInstancesClientListByResourceGroupResponse, error) {
		return armsql.ManagedInstancesClientListByResourceGroupResponse{
			ManagedInstanceListResult: armsql.ManagedInstanceListResult{
				Value: m.ResourceGroupServers,
			},
		}, nil
	})
}

type ARMMySQLFlexServerMock struct {
	NoAuth  bool
	Servers []*armmysqlflexibleservers.Server
}

func (m *ARMMySQLFlexServerMock) NewListPager(_ *armmysqlflexibleservers.ServersClientListOptions) *runtime.Pager[armmysqlflexibleservers.ServersClientListResponse] {
	return newPagerHelper(m.NoAuth, func() (armmysqlflexibleservers.ServersClientListResponse, error) {
		return armmysqlflexibleservers.ServersClientListResponse{
			ServerListResult: armmysqlflexibleservers.ServerListResult{
				Value: m.Servers,
			},
		}, nil
	})
}

func (m *ARMMySQLFlexServerMock) NewListByResourceGroupPager(group string, _ *armmysqlflexibleservers.ServersClientListByResourceGroupOptions) *runtime.Pager[armmysqlflexibleservers.ServersClientListByResourceGroupResponse] {
	return newPagerHelper(m.NoAuth, func() (armmysqlflexibleservers.ServersClientListByResourceGroupResponse, error) {
		var servers []*armmysqlflexibleservers.Server
		for _, server := range m.Servers {
			id, err := arm.ParseResourceID(StringVal(server.ID))
			if err != nil {
				return armmysqlflexibleservers.ServersClientListByResourceGroupResponse{}, trace.Wrap(err)
			}
			if group == id.ResourceGroupName {
				servers = append(servers, server)
			}
		}
		if len(servers) == 0 {
			return armmysqlflexibleservers.ServersClientListByResourceGroupResponse{}, trace.NotFound("no resources found")
		}
		return armmysqlflexibleservers.ServersClientListByResourceGroupResponse{
			ServerListResult: armmysqlflexibleservers.ServerListResult{
				Value: servers,
			},
		}, nil
	})
}

type ARMPostgresFlexServerMock struct {
	NoAuth  bool
	Servers []*armpostgresqlflexibleservers.Server
}

func (m *ARMPostgresFlexServerMock) NewListPager(_ *armpostgresqlflexibleservers.ServersClientListOptions) *runtime.Pager[armpostgresqlflexibleservers.ServersClientListResponse] {
	return newPagerHelper(m.NoAuth, func() (armpostgresqlflexibleservers.ServersClientListResponse, error) {
		return armpostgresqlflexibleservers.ServersClientListResponse{
			ServerListResult: armpostgresqlflexibleservers.ServerListResult{
				Value: m.Servers,
			},
		}, nil
	})
}

func (m *ARMPostgresFlexServerMock) NewListByResourceGroupPager(group string, _ *armpostgresqlflexibleservers.ServersClientListByResourceGroupOptions) *runtime.Pager[armpostgresqlflexibleservers.ServersClientListByResourceGroupResponse] {
	return newPagerHelper(m.NoAuth, func() (armpostgresqlflexibleservers.ServersClientListByResourceGroupResponse, error) {
		var servers []*armpostgresqlflexibleservers.Server
		for _, server := range m.Servers {
			id, err := arm.ParseResourceID(StringVal(server.ID))
			if err != nil {
				return armpostgresqlflexibleservers.ServersClientListByResourceGroupResponse{}, trace.Wrap(err)
			}
			if group == id.ResourceGroupName {
				servers = append(servers, server)
			}
		}
		if len(servers) == 0 {
			return armpostgresqlflexibleservers.ServersClientListByResourceGroupResponse{}, trace.NotFound("no resources found")
		}
		return armpostgresqlflexibleservers.ServersClientListByResourceGroupResponse{
			ServerListResult: armpostgresqlflexibleservers.ServerListResult{
				Value: servers,
			},
		}, nil
	})
}

// ARMUserAssignedIdentitiesMock implements ARMUserAssignedIdentities.
type ARMUserAssignedIdentitiesMock struct {
	identitiesMap map[string]armmsi.Identity
}

// NewARMUserAssignedIdentitiesMock creates a new ARMUserAssignedIdentitiesMock.
func NewARMUserAssignedIdentitiesMock(identities ...armmsi.Identity) *ARMUserAssignedIdentitiesMock {
	identitiesMap := make(map[string]armmsi.Identity)
	for _, identity := range identities {
		id, err := arm.ParseResourceID(*identity.ID)
		if err == nil {
			identitiesMap[id.ResourceGroupName+"+"+id.Name] = identity
		} else {
			slog.With("error", err).WarnContext(context.Background(), "Failed to add identity to mock.")
		}
	}
	return &ARMUserAssignedIdentitiesMock{
		identitiesMap: identitiesMap,
	}
}

func (m *ARMUserAssignedIdentitiesMock) Get(ctx context.Context, resourceGroupName, resourceName string, options *armmsi.UserAssignedIdentitiesClientGetOptions) (armmsi.UserAssignedIdentitiesClientGetResponse, error) {
	if m == nil || m.identitiesMap == nil {
		return armmsi.UserAssignedIdentitiesClientGetResponse{}, trace.AccessDenied("access denied")
	}

	identity, found := m.identitiesMap[resourceGroupName+"+"+resourceName]
	if !found {
		return armmsi.UserAssignedIdentitiesClientGetResponse{}, trace.NotFound("%s of group %s not found", resourceName, resourceGroupName)
	}
	return armmsi.UserAssignedIdentitiesClientGetResponse{
		Identity: identity,
	}, nil
}

// NewUserAssignedIdentity creates an armmsi.Identity.
func NewUserAssignedIdentity(subscription, resourceGroupName, resourceName, clientID string) armmsi.Identity {
	id := fmt.Sprintf("/subscriptions/%s/resourcegroups/%s/providers/Microsoft.ManagedIdentity/userAssignedIdentities/%s", subscription, resourceGroupName, resourceName)
	return armmsi.Identity{
		ID:   &id,
		Name: &resourceName,
		Properties: &armmsi.UserAssignedIdentityProperties{
			ClientID: &clientID,
		},
	}
}
