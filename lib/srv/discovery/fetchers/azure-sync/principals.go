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
	"slices"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore" //nolint:unused // used in a dependent PR
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
)

const groupType = "#microsoft.graph.group"                       //nolint:unused // used in a dependent PR
const defaultGraphScope = "https://graph.microsoft.com/.default" //nolint:unused // used in a dependent PR

// fetchPrincipals fetches the Azure principals (users, groups, and service principals) using the Graph API
func fetchPrincipals(ctx context.Context, subscriptionID string, cred azcore.TokenCredential) ([]*accessgraphv1alpha.AzurePrincipal, error) { //nolint:unused // used in a dependent PR
	// Get the graph client
	scopes := []string{defaultGraphScope}
	token, err := cred.GetToken(ctx, policy.TokenRequestOptions{Scopes: scopes})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cli := NewGraphClient(token)

	// Fetch the users, groups, and managed identities
	users, err := cli.ListUsers(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	groups, err := cli.ListGroups(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	svcPrincipals, err := cli.ListServicePrincipals(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	principals := slices.Concat(users, groups, svcPrincipals)

	// Return the users as protobuf messages
	pbPrincipals := make([]*accessgraphv1alpha.AzurePrincipal, 0, len(principals))
	for _, principal := range principals {
		// Extract group membership
		memberOf := make([]string, 0)
		for _, member := range principal.MemberOf {
			if member.Type == groupType {
				memberOf = append(memberOf, member.ID)
			}
		}
		// Create the protobuf principal and append it to the list
		pbPrincipal := &accessgraphv1alpha.AzurePrincipal{
			Id:             principal.ID,
			SubscriptionId: subscriptionID,
			LastSyncTime:   timestamppb.Now(),
			DisplayName:    principal.Name,
			MemberOf:       memberOf,
		}
		pbPrincipals = append(pbPrincipals, pbPrincipal)
	}
	return pbPrincipals, nil
}
