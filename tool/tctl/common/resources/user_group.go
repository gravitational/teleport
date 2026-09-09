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

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
)

type userGroupCollection struct {
	userGroups []types.UserGroup
}

func (c *userGroupCollection) Resources() []types.Resource {
	r := make([]types.Resource, len(c.userGroups))
	for i, resource := range c.userGroups {
		r[i] = resource
	}
	return r
}

func (c *userGroupCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "Origin"})
	for _, userGroup := range c.userGroups {
		t.AddRow([]string{
			userGroup.GetName(),
			userGroup.Origin(),
		})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func userGroupHandler() Handler {
	return Handler{
		getHandler:    getUserGroup,
		deleteHandler: deleteUserGroup,
		description:   "Represents a group of users imported from an identity provider.",
	}
}

func getUserGroup(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	if ref.Name != "" {
		userGroup, err := client.GetUserGroup(ctx, ref.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &userGroupCollection{userGroups: []types.UserGroup{userGroup}}, nil
	}

	userGroups, err := stream.Collect(clientutils.Resources(ctx, client.ListUserGroups))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &userGroupCollection{userGroups: userGroups}, nil
}

func deleteUserGroup(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.DeleteUserGroup(ctx, ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("User group %q has been deleted\n", ref.Name)
	return nil
}
