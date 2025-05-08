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

package clusters

import (
	"context"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/trace"
)

// ListResources returns a paginated list of requested resourceKind (singular kind).
// Different from unified resources where this request does not support multi kinds in a response.
// ListResources supports querying for resource kinds that are not supported by unified resources (eg: db_server)
func ListResources[T types.ResourceWithLabels](ctx context.Context, r *api.ListResourcesRequest, authClient authclient.ClientI, resourceKind string) (apiclient.ResourcePage[T], error) {
	var (
		page apiclient.ResourcePage[T]
		err  error
	)

	req := &proto.ListResourcesRequest{
		ResourceType:        resourceKind,
		Limit:               r.Limit,
		StartKey:            r.StartKey,
		PredicateExpression: r.PredicateExpression,
		UseSearchAsRoles:    r.UseSearchAsRoles,
	}

	err = AddMetadataToRetryableError(ctx, func() error {
		page, err = apiclient.GetResourcePage[T](ctx, authClient, req)
		if err != nil {
			return trace.Wrap(err)
		}

		return nil
	})

	return page, trace.Wrap(err)
}
