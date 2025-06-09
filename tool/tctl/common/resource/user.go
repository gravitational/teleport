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

	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/tctl/common/resource/collections"
)

var user = resource{
	getHandler:    getUser,
	createHandler: createUser,
	updateHandler: updateUser,
	deleteHandler: deleteUser,
}

func getUser(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	if ref.Name == "" {
		users, err := client.GetUsers(ctx, opts.withSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return collections.NewUserCollection(users), nil
	}
	user, err := client.GetUser(ctx, ref.Name, opts.withSecrets)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return collections.NewUserCollection(services.Users{user}), nil
}

// createUser implements `tctl create user.yaml` command.
func createUser(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
	user, err := services.UnmarshalUser(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	userName := user.GetName()
	existingUser, err := client.GetUser(ctx, userName, false)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	exists := (err == nil)

	if exists {
		if !opts.force {
			return trace.AlreadyExists("user %q already exists", userName)
		}

		// Unmarshalling user sets createdBy to zero values which will overwrite existing data.
		// This field should not be allowed to be overwritten.
		user.SetCreatedBy(existingUser.GetCreatedBy())

		if _, err := client.UpsertUser(ctx, user); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("user %q has been updated\n", userName)

	} else {
		if _, err := client.CreateUser(ctx, user); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("user %q has been created\n", userName)
	}

	return nil
}

// updateUser implements `tctl create user.yaml` command.
func updateUser(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
	user, err := services.UnmarshalUser(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if _, err := client.UpdateUser(ctx, user); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("user %q has been updated\n", user.GetName())

	return nil
}

func deleteUser(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.DeleteUser(ctx, ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("user %q has been deleted\n", ref.Name)
	return nil
}
