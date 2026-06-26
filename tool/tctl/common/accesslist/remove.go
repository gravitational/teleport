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

package accesslist

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/utils"
)

// Remove deletes an access list. If the list was created with a preset
// (which automatically creates supporting roles), after deletion those
// roles will be listed for the user to optionally manually
// delete with `tctl rm roles/<name>`.
//
// Roles are not automatically deleted for the user since nothing stops a user
// from assigning these roles to other users or referencing them in other roles.
// Deleting a role assigned to other resources will lock a user out of their account.
func (c *Command) Remove(ctx context.Context, client *authclient.Client) error {
	al, err := client.AccessListClient().GetAccessList(ctx, c.accessListName)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := client.AccessListClient().DeleteAccessList(ctx, c.accessListName); err != nil {
		return trace.Wrap(err)
	}

	resp := RemoveJSONResponse{
		// TODO(kimlisa): The list of role names from GetAccessList can refer to stale roles
		// since in between the Get and Delete call, an update may have occurred.
		// Have a new rpc "DeleteAccessListV2" return the list of role names to delete instead,
		// and fall back to GetAccessList/DeleteAccessList otherwise.
		RolesToDelete: al.PresetRoleNames(),
	}

	if c.format == teleport.JSON {
		return trace.Wrap(utils.WriteJSON(c.Stdout, resp), "failed to marshal access list delete response")
	}

	fmt.Fprintf(c.Stdout, "Deleted access list %q\n", c.accessListName)
	c.printRolesToBeDeleted(resp.RolesToDelete)
	return nil
}

// printRolesToBeDeleted prints the guidance block listing preset roles that are
// no longer used by an access list, shared by `acl rm` and `acl update` so the
// output is identical. No-op when there are no such roles.
func (c *Command) printRolesToBeDeleted(roles []string) {
	if len(roles) == 0 {
		return
	}
	fmt.Fprintln(c.Stdout)
	fmt.Fprintln(c.Stdout, "The following roles are no longer used by this access list:")
	for _, name := range roles {
		fmt.Fprintf(c.Stdout, "  - %s\n", name)
	}
	fmt.Fprintln(c.Stdout)
	fmt.Fprintln(c.Stdout, "These roles may still be assigned to users or referenced by other roles.")
	fmt.Fprintln(c.Stdout, "Verify each role is unused before deleting it with:")
	fmt.Fprintln(c.Stdout, "  tctl rm roles/<name>")
}

// RemoveJSONResponse is a structured response when `format=json`
// is requested.
type RemoveJSONResponse struct {
	// RolesToDelete contains role names that are no longer
	// valid for the deleted access list.
	RolesToDelete []string `json:"roles_to_delete,omitempty"`
}
