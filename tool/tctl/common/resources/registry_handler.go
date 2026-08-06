/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package resources

import (
	"context"

	"github.com/gravitational/trace"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/resourceregistry"
)

func registeredClient[T any](client *authclient.Client, spec resourceregistry.Spec[T, resourceregistry.NameID]) (resourceregistry.Client[T, resourceregistry.NameID], error) {
	resourceClient, err := spec.Client(client)
	return resourceClient, trace.Wrap(err)
}

func decodeRegisteredResource[T any](raw services.UnknownResource, spec resourceregistry.Spec[T, resourceregistry.NameID]) (T, error) {
	resource, err := spec.Unmarshal(raw.Raw, services.DisallowUnknown())
	return resource, trace.Wrap(err)
}

func getRegisteredResources[T any](
	ctx context.Context,
	client *authclient.Client,
	ref services.Ref,
	spec resourceregistry.Spec[T, resourceregistry.NameID],
	collection func([]T) Collection,
) (Collection, error) {
	resourceClient, err := registeredClient(client, spec)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if ref.Name != "" {
		resource, err := resourceClient.Get(ctx, resourceregistry.NameID(ref.Name))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return collection([]T{resource}), nil
	}

	var resources []T
	for token := ""; ; {
		page, next, err := resourceClient.List(ctx, resourceregistry.Page{
			Size:  apidefaults.DefaultChunkSize,
			Token: token,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		resources = append(resources, page...)
		if next == "" {
			break
		}
		token = next
	}
	return collection(resources), nil
}

func upsertRegisteredResource[T any](
	ctx context.Context,
	client resourceregistry.Client[T, resourceregistry.NameID],
	resource T,
) (T, error) {
	upserter, ok := client.(resourceregistry.Upserter[T])
	if !ok {
		return client.Update(ctx, resource)
	}
	upserted, err := upserter.Upsert(ctx, resource)
	return upserted, trace.Wrap(err)
}
