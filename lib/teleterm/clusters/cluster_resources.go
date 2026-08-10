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

	"github.com/gravitational/trace"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	"github.com/gravitational/teleport/lib/auth/authclient"
)

// listResources returns a paginated list of requested resourceKind (singular kind).
// Unlike unified resources, this request does not support multiple kinds in a single response.
// listResources supports querying for resource kinds that are not supported by unified resources.
// In addition, listResources does not de-duplicate resources like ListUnifiedResources does.
func listResources[T types.ResourceWithLabels](ctx context.Context, params *api.ListResourcesParams, authClient authclient.ClientI, resourceKind string) (apiclient.ResourcePage[T], error) {
	var (
		page apiclient.ResourcePage[T]
		err  error
	)

	req := &proto.ListResourcesRequest{
		ResourceType:        resourceKind,
		Limit:               params.Limit,
		StartKey:            params.StartKey,
		PredicateExpression: params.PredicateExpression,
		UseSearchAsRoles:    params.UseSearchAsRoles,
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
