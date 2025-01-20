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

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/msgraph"
)

type dirObjMetadata struct { //nolint:unused // invoked in a dependent PR
	objectType string
}

type queryResult struct { //nolint:unused // invoked in a dependent PR
	metadata dirObjMetadata
	dirObj   msgraph.DirectoryObject
}

// fetchPrincipals fetches the Azure principals (users, groups, and service principals) using the Graph API
func fetchPrincipals(ctx context.Context, subscriptionID string, cli *msgraph.Client) ([]*accessgraphv1alpha.AzurePrincipal, error) { //nolint: unused // invoked in a dependent PR
	// Fetch the users, groups, and service principals as directory objects
	var queryResults []queryResult
	err := cli.IterateUsers(ctx, func(user *msgraph.User) bool {
		res := queryResult{metadata: dirObjMetadata{objectType: "user"}, dirObj: user.DirectoryObject}
		queryResults = append(queryResults, res)
		return true
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = cli.IterateGroups(ctx, func(group *msgraph.Group) bool {
		res := queryResult{metadata: dirObjMetadata{objectType: "group"}, dirObj: group.DirectoryObject}
		queryResults = append(queryResults, res)
		return true
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = cli.IterateServicePrincipals(ctx, func(servicePrincipal *msgraph.ServicePrincipal) bool {
		res := queryResult{metadata: dirObjMetadata{objectType: "servicePrincipal"}, dirObj: servicePrincipal.DirectoryObject}
		queryResults = append(queryResults, res)
		return true
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Return the users, groups, and service principals as protobuf messages
	var fetchErrs []error
	var pbPrincipals []*accessgraphv1alpha.AzurePrincipal
	for _, res := range queryResults {
		if res.dirObj.ID == nil || res.dirObj.DisplayName == nil {
			fetchErrs = append(fetchErrs,
				trace.BadParameter("nil values on msgraph directory object: %v", res.dirObj))
			continue
		}
		pbPrincipals = append(pbPrincipals, &accessgraphv1alpha.AzurePrincipal{
			Id:             *res.dirObj.ID,
			SubscriptionId: subscriptionID,
			LastSyncTime:   timestamppb.Now(),
			DisplayName:    *res.dirObj.DisplayName,
			ObjectType:     res.metadata.objectType,
		})
	}
	return pbPrincipals, trace.NewAggregate(fetchErrs...)
}
