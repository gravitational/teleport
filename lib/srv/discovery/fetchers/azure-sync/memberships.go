/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
	"golang.org/x/sync/errgroup"

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/msgraph"
)

const parallelism = 10 //nolint:unused // invoked in a dependent PR

// expandMemberships adds membership data to AzurePrincipal objects by querying the Graph API for group memberships
func expandMemberships(ctx context.Context, cli *msgraph.Client, principals []*accessgraphv1alpha.AzurePrincipal) ([]*accessgraphv1alpha.AzurePrincipal, error) { //nolint:unused // invoked in a dependent PR
	// Map principals by ID
	var principalsMap = make(map[string]*accessgraphv1alpha.AzurePrincipal)
	for _, principal := range principals {
		principalsMap[principal.Id] = principal
	}
	// Iterate through the Azure groups and add the group ID as a membership for its corresponding principal
	eg, _ := errgroup.WithContext(ctx)
	eg.SetLimit(parallelism)
	errCh := make(chan error, len(principals))
	for _, principal := range principals {
		if principal.ObjectType != "group" {
			continue
		}
		group := principal
		eg.Go(func() error {
			err := cli.IterateGroupMembers(ctx, group.Id, func(member msgraph.GroupMember) bool {
				if memberPrincipal, ok := principalsMap[*member.GetID()]; ok {
					memberPrincipal.MemberOf = append(memberPrincipal.MemberOf, group.Id)
				}
				return true
			})
			if err != nil {
				errCh <- err
			}
			return nil
		})
	}
	_ = eg.Wait()
	close(errCh)
	return principals, trace.NewAggregateFromChannel(errCh, ctx)
}
