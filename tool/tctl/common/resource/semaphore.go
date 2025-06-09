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

var semaphore = resource{
	getHandler:    getSemaphore,
	deleteHandler: deleteSemaphore,
}

func getSemaphore(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	sems, err := client.GetSemaphores(ctx, types.SemaphoreFilter{
		SemaphoreKind: ref.SubKind,
		SemaphoreName: ref.Name,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return collections.NewSemaphoreCollection(sems), nil
}

func deleteSemaphore(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if ref.SubKind == "" || ref.Name == "" {
		return trace.BadParameter(
			"full semaphore path must be specified (e.g. '%s/%s/alice@example.com')",
			types.KindSemaphore, types.SemaphoreKindConnection,
		)
	}
	err := client.DeleteSemaphore(ctx, types.SemaphoreFilter{
		SemaphoreKind: ref.SubKind,
		SemaphoreName: ref.Name,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("semaphore '%s/%s' has been deleted\n", ref.SubKind, ref.Name)
	return nil
}
