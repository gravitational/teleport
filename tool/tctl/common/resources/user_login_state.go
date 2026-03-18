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
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/userloginstate"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
)

type userLoginStateCollection struct {
	states []*userloginstate.UserLoginState
}

func (c *userLoginStateCollection) Resources() []types.Resource {
	resources := make([]types.Resource, len(c.states))
	for i, s := range c.states {
		resources[i] = s
	}
	return resources
}

func (c *userLoginStateCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "Roles", "User Type"})
	for _, s := range c.states {
		t.AddRow([]string{
			s.GetName(),
			strings.Join(s.GetRoles(), ", "),
			string(s.GetUserType()),
		})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func userLoginStateHandler() Handler {
	return Handler{
		getHandler:    getUserLoginState,
		deleteHandler: deleteUserLoginState,
		singleton:     false,
		mfaRequired:   false,
		description:   "The ephemeral login state of a Teleport user.",
	}
}

func getUserLoginState(
	ctx context.Context,
	client *authclient.Client,
	ref services.Ref,
	opts GetOpts,
) (Collection, error) {
	c := client.UserLoginStateClient()
	if ref.Name != "" {
		state, err := c.GetUserLoginState(ctx, ref.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &userLoginStateCollection{states: []*userloginstate.UserLoginState{state}}, nil
	}

	states, err := c.GetUserLoginStates(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &userLoginStateCollection{states: states}, nil
}

func deleteUserLoginState(
	ctx context.Context,
	client *authclient.Client,
	ref services.Ref,
) error {
	if err := client.UserLoginStateClient().DeleteUserLoginState(ctx, ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("user login state %q has been deleted\n", ref.Name)
	return nil
}
