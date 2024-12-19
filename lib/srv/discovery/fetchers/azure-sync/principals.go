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
	"github.com/gravitational/teleport/lib/msgraph"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
)

// fetchPrincipals fetches the Azure principals (users, groups, and service principals) using the Graph API
func fetchPrincipals(ctx context.Context, subscriptionID string, cli *msgraph.Client) ([]*accessgraphv1alpha.AzurePrincipal, error) {
	// Fetch the users, groups, and service principals
	var users []*msgraph.User
	err := cli.IterateUsers(ctx, func(user *msgraph.User) bool {
		users = append(users, user)
		return true
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var groups []*msgraph.Group
	err = cli.IterateGroups(ctx, func(group *msgraph.Group) bool {
		groups = append(groups, group)
		return true
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var servicePrincipals []*msgraph.ServicePrincipal
	err = cli.IterateServicePrincipals(ctx, func(servicePrincipal *msgraph.ServicePrincipal) bool {
		servicePrincipals = append(servicePrincipals, servicePrincipal)
		return true
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Return the users, groups, and service principals as protobuf messages
	var pbPrincipals []*accessgraphv1alpha.AzurePrincipal
	for _, user := range users {
		var memberOf []string
		for _, member := range user.MemberOf {
			memberOf = append(memberOf, member.ID)
		}
		pbPrincipals = append(pbPrincipals, &accessgraphv1alpha.AzurePrincipal{
			Id:             *user.ID,
			SubscriptionId: subscriptionID,
			LastSyncTime:   timestamppb.Now(),
			DisplayName:    *user.DisplayName,
			MemberOf:       memberOf,
			ObjectType:     "user",
		})
	}
	for _, group := range groups {
		var memberOf []string
		for _, member := range group.MemberOf {
			memberOf = append(memberOf, member.ID)
		}
		pbPrincipals = append(pbPrincipals, &accessgraphv1alpha.AzurePrincipal{
			Id:             *group.ID,
			SubscriptionId: subscriptionID,
			LastSyncTime:   timestamppb.Now(),
			DisplayName:    *group.DisplayName,
			MemberOf:       memberOf,
			ObjectType:     "group",
		})
	}
	for _, sp := range servicePrincipals {
		var memberOf []string
		for _, member := range sp.MemberOf {
			memberOf = append(memberOf, member.ID)
		}
		pbPrincipals = append(pbPrincipals, &accessgraphv1alpha.AzurePrincipal{
			Id:             *sp.ID,
			SubscriptionId: subscriptionID,
			LastSyncTime:   timestamppb.Now(),
			DisplayName:    *sp.DisplayName,
			MemberOf:       memberOf,
			ObjectType:     "servicePrincipal",
		})
	}

	return pbPrincipals, nil
}
