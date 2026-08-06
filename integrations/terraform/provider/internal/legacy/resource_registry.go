// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package legacy

import (
	"context"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services/resourceregistry"
)

func roleRegistrySpec() resourceregistry.Spec[types.Role, resourceregistry.NameID] {
	return resourceregistry.MustGet[types.Role, resourceregistry.NameID](
		resourceregistry.Default(),
		types.KindRole,
	)
}

func roleRegistryClient(p Provider) (resourceregistry.Client[types.Role, resourceregistry.NameID], error) {
	return roleRegistrySpec().Client(p.Client())
}

func registryNameID(name string) resourceregistry.NameID {
	return resourceregistry.NameID(name)
}

func upsertRegistryResource[T any](
	ctx context.Context,
	client resourceregistry.Client[T, resourceregistry.NameID],
	resource T,
) (T, error) {
	if upserter, ok := client.(resourceregistry.Upserter[T]); ok {
		return upserter.Upsert(ctx, resource)
	}
	return client.Update(ctx, resource)
}
