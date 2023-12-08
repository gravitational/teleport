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
