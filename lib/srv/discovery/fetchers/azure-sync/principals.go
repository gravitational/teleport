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
	"net/url"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/msgraph"
)

// fetchPrincipals fetches the Azure principals (users, groups, and service principals) using the Graph API
func fetchPrincipals(ctx context.Context, subscriptionID string, cli *msgraph.Client) ([]*accessgraphv1alpha.AzurePrincipal, error) { //nolint: unused // invoked in a dependent PR
	var params = &url.Values{
		"$expand": []string{"memberOf"},
	}

	// Fetch the users, groups, and service principals as directory objects
	var dirObjs []msgraph.DirectoryObject
	err := cli.IterateUsers(ctx, params, func(user *msgraph.User) bool {
		dirObjs = append(dirObjs, user.DirectoryObject)
		return true
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = cli.IterateGroups(ctx, params, func(group *msgraph.Group) bool {
		dirObjs = append(dirObjs, group.DirectoryObject)
		return true
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = cli.IterateServicePrincipals(ctx, params, func(servicePrincipal *msgraph.ServicePrincipal) bool {
		dirObjs = append(dirObjs, servicePrincipal.DirectoryObject)
		return true
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Return the users, groups, and service principals as protobuf messages
	var fetchErrs []error
	var pbPrincipals []*accessgraphv1alpha.AzurePrincipal
	for _, dirObj := range dirObjs {
		if dirObj.ID == nil || dirObj.DisplayName == nil {
			fetchErrs = append(fetchErrs, trace.BadParameter("nil values on msgraph directory object: %v", dirObj))
			continue
		}
		var memberOf []string
		for _, member := range dirObj.MemberOf {
			memberOf = append(memberOf, member.ID)
		}
		pbPrincipals = append(pbPrincipals, &accessgraphv1alpha.AzurePrincipal{
			Id:             *dirObj.ID,
			SubscriptionId: subscriptionID,
			LastSyncTime:   timestamppb.Now(),
			DisplayName:    *dirObj.DisplayName,
			MemberOf:       memberOf,
			ObjectType:     "user",
		})
	}
	return pbPrincipals, trace.NewAggregate(fetchErrs...)
}
