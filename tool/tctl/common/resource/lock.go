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

var lock = resource{
	getHandler:    getLock,
	createHandler: createLock,
	deleteHandler: deleteLock,
}

func getLock(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	if ref.Name == "" {
		locks, err := client.GetLocks(ctx, false)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return collections.NewLockCollection(locks), nil
	}
	name := ref.Name
	if ref.SubKind != "" {
		name = ref.SubKind + "/" + name
	}
	lock, err := client.GetLock(ctx, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return collections.NewLockCollection([]types.Lock{lock}), nil
}

// createLock implements `tctl create lock.yaml` command.
func createLock(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
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

	exists := (err == nil)
	if !opts.force && exists {
		return trace.AlreadyExists("lock %q already exists", name)
	}

	if err := client.UpsertLock(ctx, lock); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("lock %q has been %s\n", name, UpsertVerb(exists, opts.force))
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
