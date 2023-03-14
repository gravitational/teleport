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

// armManagedSQLServerClient  is an interface defines a subset of functions of
// armsql.ManagedInstancesClient.
type armSQLManagedInstancesClient interface {
	NewListPager(options *armsql.ManagedInstancesClientListOptions) *runtime.Pager[armsql.ManagedInstancesClientListResponse]
	NewListByResourceGroupPager(resourceGroupName string, options *armsql.ManagedInstancesClientListByResourceGroupOptions) *runtime.Pager[armsql.ManagedInstancesClientListByResourceGroupResponse]
}

// managedSQLClient is an Azure Managed SQL Server client.
type managedSQLClient struct {
	api armSQLManagedInstancesClient
}

// NewSQLClient creates a new Azure SQL Server client by subscription and
// credentials.
func NewManagedSQLClient(subscription string, cred azcore.TokenCredential, options *arm.ClientOptions) (ManagedSQLServerClient, error) {
	api, err := armsql.NewManagedInstancesClient(subscription, cred, options)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &managedSQLClient{api}, nil
}

// NewSQLClientByAPI creates a new Azure SQL Serverclient by ARM API client.
func NewManagedSQLClientByAPI(api armSQLManagedInstancesClient) ManagedSQLServerClient {
	return &managedSQLClient{api}
}

func (c *managedSQLClient) ListAll(ctx context.Context) ([]*armsql.ManagedInstance, error) {
	pager := c.api.NewListPager(&armsql.ManagedInstancesClientListOptions{})

	var servers []*armsql.ManagedInstance
	for pageNum := 0; pager.More(); pageNum++ {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(ConvertResponseError(err))
		}
		servers = append(servers, page.Value...)
	}

	return servers, nil
}

func (c *managedSQLClient) ListWithinGroup(ctx context.Context, group string) ([]*armsql.ManagedInstance, error) {
	pager := c.api.NewListByResourceGroupPager(group, &armsql.ManagedInstancesClientListByResourceGroupOptions{})

	var servers []*armsql.ManagedInstance
	for pageNum := 0; pager.More(); pageNum++ {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(ConvertResponseError(err))
		}
		servers = append(servers, page.Value...)
	}

	return servers, nil
}
