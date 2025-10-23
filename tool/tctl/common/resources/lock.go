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

package resources

import (
	"context"
	"fmt"
	"io"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
)

type lockCollection struct {
	locks []types.Lock
}

func (c *lockCollection) Resources() []types.Resource {
	r := make([]types.Resource, 0, len(c.locks))
	for _, resource := range c.locks {
		r = append(r, resource)
	}
	return r
}

func (c *lockCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"ID", "Target", "Message", "Expires"})
	for _, lock := range c.locks {
		target := lock.Target()
		expires := "never"
		if lock.LockExpiry() != nil {
			expires = apiutils.HumanTimeFormat(*lock.LockExpiry())
		}
		t.AddRow([]string{lock.GetName(), target.String(), lock.Message(), expires})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func lockHandler() Handler {
	return Handler{
		getHandler:    getLock,
		createHandler: createLock,
		deleteHandler: deleteLock,
		singleton:     false,
		mfaRequired:   false,
		description:   "Prevents a user, node or bot from interacting with Teleport.",
	}
}

func getLock(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	if ref.Name == "" {
		locks, err := client.GetLocks(ctx, false)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &lockCollection{locks: locks}, nil
	}
	name := ref.Name
	if ref.SubKind != "" {
		name = ref.SubKind + "/" + name
	}
	lock, err := client.GetLock(ctx, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &lockCollection{locks: []types.Lock{lock}}, nil

}

func createLock(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	lock, err := services.UnmarshalLock(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	// Check if a lock of the name already exists.
	name := lock.GetName()
	_, err = client.GetLock(ctx, name)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	exists := err == nil
	if !opts.Force && exists {
		return trace.AlreadyExists("lock %q already exists", name)
	}

	if err := client.UpsertLock(ctx, lock); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("lock %q has been %s\n", name, upsertVerb(exists, opts.Force))
	return nil
}

func deleteLock(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	name := ref.Name
	if ref.SubKind != "" {
		name = ref.SubKind + "/" + name
	}
	if err := client.DeleteLock(ctx, name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("lock %q has been deleted\n", name)
	return nil
}
