// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package resources

import (
	"context"
	"fmt"
	"io"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
)

type userCollection struct {
	users []types.User
}

// NewUserCollection creates a [Collection] over the provided users.
func NewUserCollection(users []types.User) Collection {
	return &userCollection{users: users}
}

func (u *userCollection) Resources() []types.Resource {
	r := make([]types.Resource, 0, len(u.users))
	for _, resource := range u.users {
		r = append(r, resource)
	}
	return r
}

func (u *userCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"User"})
	for _, user := range u.users {
		t.AddRow([]string{user.GetName()})
	}
	fmt.Println(t.AsBuffer().String())
	return nil
}

func userHandler() Handler {
	return Handler{
		getHandler:    getUser,
		createHandler: createUser,
		updateHandler: updateUser,
		deleteHandler: deleteUser,
		singleton:     false,
		mfaRequired:   true,
		description:   "A human user within Teleport.",
	}
}

// createUser implements `tctl create user.yaml` command.
func createUser(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
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
		if !opts.Force {
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

// getUser implements `tctl get user/russell` command.
func getUser(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	if ref.Name == "" {
		users, err := client.GetUsers(ctx, opts.WithSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &userCollection{users: users}, nil
	}
	user, err := client.GetUser(ctx, ref.Name, opts.WithSecrets)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &userCollection{users: services.Users{user}}, nil
}

// updateUser implements `tctl create user.yaml` command.
func updateUser(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
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

// deleteUser implements `tctl delete user/russell` command.
func deleteUser(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.DeleteUser(ctx, ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("user %q has been deleted\n", ref.Name)
	return nil
}
