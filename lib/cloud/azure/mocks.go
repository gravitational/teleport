/*
Copyright 2022 Gravitational, Inc.

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

package azure

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/mysql/armmysql"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresql"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/redis/armredis/v2"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/redisenterprise/armredisenterprise"
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
	Token  string
	NoAuth bool
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

// ARMRedisEnterpriseDatabaseMock mocks armRedisEnterpriseDatabaseClient.
type ARMRedisEnterpriseDatabaseMock struct {
	Token                string
	TokensByDatabaseName map[string]string
	NoAuth               bool
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
