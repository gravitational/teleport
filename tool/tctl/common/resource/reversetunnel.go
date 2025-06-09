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

package resource

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/tctl/common/resource/collections"
)

var reverseTunnel = resource{
	getHandler:    getReverseTunnel,
	deleteHandler: deleteReverseTunnel,
}

func getReverseTunnel(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	if ref.Name != "" {
		return nil, trace.BadParameter("reverse tunnel cannot be searched by name")
	}
	var tunnels []types.ReverseTunnel
	var nextToken string
	for {
		var page []types.ReverseTunnel
		var err error

		const defaultPageSize = 0
		page, nextToken, err = client.ListReverseTunnels(ctx, defaultPageSize, nextToken)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		tunnels = append(tunnels, page...)
		if nextToken == "" {
			break
		}
	}
	return collections.NewReverseTunnelCollection(tunnels), nil
}

func deleteReverseTunnel(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.DeleteTrustedCluster(ctx, ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("trusted cluster %q has been deleted\n", ref.Name)
	return nil
}
