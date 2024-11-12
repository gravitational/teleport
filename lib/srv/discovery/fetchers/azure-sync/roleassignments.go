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

package azure_sync

import (
	"context"
	"fmt"
	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (a *Fetcher) fetchRoleAssignments(ctx context.Context) ([]*accessgraphv1alpha.AzureRoleAssignment, error) {
	// List the role definitions
	roleAssigns, err := a.roleAssignClient.ListRoleAssignments(ctx, fmt.Sprintf("/subscriptions/%s", a.SubscriptionID))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Convert to protobuf format
	pbRoleAssigns := make([]*accessgraphv1alpha.AzureRoleAssignment, 0, len(roleAssigns))
	for _, roleAssign := range roleAssigns {
		pbRoleAssign := &accessgraphv1alpha.AzureRoleAssignment{
			Id:               *roleAssign.ID,
			SubscriptionId:   a.SubscriptionID,
			LastSyncTime:     timestamppb.Now(),
			PrincipalId:      *roleAssign.Properties.PrincipalID,
			RoleDefinitionId: *roleAssign.Properties.RoleDefinitionID,
			Scope:            *roleAssign.Properties.Scope,
		}
		pbRoleAssigns = append(pbRoleAssigns, pbRoleAssign)
	}
	return pbRoleAssigns, nil
}
