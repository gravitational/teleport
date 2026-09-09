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
	"fmt"
	"io"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
)

type semaphoreCollection struct {
	sems []types.Semaphore
}

func (c *semaphoreCollection) Resources() (r []types.Resource) {
	for _, resource := range c.sems {
		r = append(r, resource)
	}
	return r
}

func (c *semaphoreCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Kind", "Name", "LeaseID", "Holder", "Expires"})
	for _, sem := range c.sems {
		for _, ref := range sem.LeaseRefs() {
			t.AddRow([]string{
				sem.GetSubKind(), sem.GetName(), ref.LeaseID, ref.Holder, ref.Expires.Format(time.RFC822),
			})
		}
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func semaphoreHandler() Handler {
	return Handler{
		getHandler:    getSemaphore,
		deleteHandler: deleteSemaphore,
		description:   "Internal resource used to limit concurrent access.",
	}
}

func getSemaphore(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	filter := types.SemaphoreFilter{
		SemaphoreKind: ref.SubKind,
		SemaphoreName: ref.Name,
	}
	sems, err := clientutils.CollectWithFallback(ctx,
		func(ctx context.Context, pageSize int, pageToken string) ([]types.Semaphore, string, error) {
			return client.ListSemaphores(ctx, pageSize, pageToken, &filter)
		},
		func(ctx context.Context) ([]types.Semaphore, error) {
			return client.GetSemaphores(ctx, filter)
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &semaphoreCollection{sems: sems}, nil
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
