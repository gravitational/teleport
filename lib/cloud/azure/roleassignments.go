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

package azure

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization/v2"
	"github.com/gravitational/trace"
)

// RoleAssignmentsClient wraps the Azure API to provide a high level subset of functionality
type RoleAssignmentsClient struct {
	cli *armauthorization.RoleAssignmentsClient
}

// NewRoleAssignmentsClient creates a new client for a given subscription and credentials
func NewRoleAssignmentsClient(subscription string, cred azcore.TokenCredential, options *arm.ClientOptions) (*RoleAssignmentsClient, error) {
	clientFactory, err := armauthorization.NewClientFactory(subscription, cred, options)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	roleDefCli := clientFactory.NewRoleAssignmentsClient()
	return &RoleAssignmentsClient{cli: roleDefCli}, nil
}

// ListRoleAssignments returns role assignments for a given scope
func (c *RoleAssignmentsClient) ListRoleAssignments(ctx context.Context, scope string) ([]*armauthorization.RoleAssignment, error) {
	pager := c.cli.NewListForScopePager(scope, nil)
	var roleDefs []*armauthorization.RoleAssignment
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		roleDefs = append(roleDefs, page.Value...)
	}
	return roleDefs, nil
}
