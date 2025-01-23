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

package azuresync

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization/v2"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
)

// RoleAssignmentsClient specifies the methods used to fetch role assignments from Azure
type RoleAssignmentsClient interface {
	ListRoleAssignments(ctx context.Context, scope string) ([]*armauthorization.RoleAssignment, error)
}

// fetchRoleAssignments fetches Azure role assignments using the Azure role assignments API
func fetchRoleAssignments(ctx context.Context, subscriptionID string, cli RoleAssignmentsClient) ([]*accessgraphv1alpha.AzureRoleAssignment, error) { //nolint:unused // invoked in a dependent PR
	// List the role definitions
	roleAssigns, err := cli.ListRoleAssignments(ctx, fmt.Sprintf("/subscriptions/%s", subscriptionID))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Convert to protobuf format
	pbRoleAssigns := make([]*accessgraphv1alpha.AzureRoleAssignment, 0, len(roleAssigns))
	var fetchErrs []error
	for _, roleAssign := range roleAssigns {
		if roleAssign.ID == nil ||
			roleAssign.Properties == nil ||
			roleAssign.Properties.PrincipalID == nil ||
			roleAssign.Properties.Scope == nil {
			fetchErrs = append(fetchErrs,
				trace.BadParameter("nil values on AzureRoleAssignment object: %v", roleAssign))
			continue
		}
		pbRoleAssign := &accessgraphv1alpha.AzureRoleAssignment{
			Id:               *roleAssign.ID,
			SubscriptionId:   subscriptionID,
			LastSyncTime:     timestamppb.Now(),
			PrincipalId:      *roleAssign.Properties.PrincipalID,
			RoleDefinitionId: *roleAssign.Properties.RoleDefinitionID,
			Scope:            *roleAssign.Properties.Scope,
		}
		pbRoleAssigns = append(pbRoleAssigns, pbRoleAssign)
	}
	return pbRoleAssigns, trace.NewAggregate(fetchErrs...)
}
