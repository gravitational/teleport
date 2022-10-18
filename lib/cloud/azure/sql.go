// Copyright 2022 Gravitational, Inc
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

package azure

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/sql/armsql"
	"github.com/gravitational/trace"
)

// armSQLServerClient is an interface defines a subset of functions of
// armsql.ServersClient.
type armSQLServerClient interface {
	NewListPager(options *armsql.ServersClientListOptions) *runtime.Pager[armsql.ServersClientListResponse]
	NewListByResourceGroupPager(resourceGroupName string, options *armsql.ServersClientListByResourceGroupOptions) *runtime.Pager[armsql.ServersClientListByResourceGroupResponse]
}

// sqlClient  is an Azure SQL Server client.
type sqlClient struct {
	api armSQLServerClient
}

// NewSQLClient creates a new Azure SQL Server client by subscription and
// credentials.
func NewSQLClient(subscription string, cred azcore.TokenCredential, options *arm.ClientOptions) (SQLServerClient, error) {
	api, err := armsql.NewServersClient(subscription, cred, options)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &sqlClient{api}, nil
}

// NewSQLClientByAPI creates a new Azure SQL Serverclient by ARM API client.
func NewSQLClientByAPI(api armSQLServerClient) SQLServerClient {
	return &sqlClient{api}
}

func (c *sqlClient) ListAll(ctx context.Context) ([]*armsql.Server, error) {
	pager := c.api.NewListPager(&armsql.ServersClientListOptions{})

	var servers []*armsql.Server
	for pageNum := 0; pager.More(); pageNum++ {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(ConvertResponseError(err))
		}
		servers = append(servers, page.Value...)
	}

	return servers, nil
}

func (c *sqlClient) ListWithinGroup(ctx context.Context, group string) ([]*armsql.Server, error) {
	pager := c.api.NewListByResourceGroupPager(group, &armsql.ServersClientListByResourceGroupOptions{})

	var servers []*armsql.Server
	for pageNum := 0; pager.More(); pageNum++ {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(ConvertResponseError(err))
		}
		servers = append(servers, page.Value...)
	}

	return servers, nil
}
