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

func expandMemberships(ctx context.Context, cli *msgraph.Client, principals []*accessgraphv1alpha.AzurePrincipal) ([]*accessgraphv1alpha.AzurePrincipal, error) { //nolint:unused // invoked in a dependent PR
	eg, _ := errgroup.WithContext(ctx)
	eg.SetLimit(parallelism)
	errCh := make(chan error, len(principals))
	for _, principal := range principals {
		eg.Go(func() error {
			err := cli.IterateUserMembership(ctx, principal.Id, func(obj *msgraph.DirectoryObject) bool {
				principal.MemberOf = append(principal.MemberOf, *obj.ID)
				return true
			})
			if err != nil {
				errCh <- err
			}
			return nil
		})
	}
	_ = eg.Wait()
	var errs []error
	for chErr := range errCh {
		errs = append(errs, chErr)
	}
	if len(errs) > 0 {
		return nil, trace.NewAggregate(errs...)
	}
	return principals, nil
}
