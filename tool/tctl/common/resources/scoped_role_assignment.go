// Teleport
// Copyright (C) 2026  Gravitational, Inc.
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
	"strings"

	"github.com/gravitational/trace"

	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/itertools/stream"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
	scopedutils "github.com/gravitational/teleport/lib/scopes/utils"
	"github.com/gravitational/teleport/lib/services"
)

type scopedRoleAssignmentCollection struct {
	roleAssignments []*scopedaccessv1.ScopedRoleAssignment
}

func NewScopedRoleAssignmentCollection(roles []*scopedaccessv1.ScopedRoleAssignment) Collection {
	return &scopedRoleAssignmentCollection{
		roleAssignments: roles,
	}
}

func (c *scopedRoleAssignmentCollection) Resources() []types.Resource {
	r := make([]types.Resource, len(c.roleAssignments))
	for i, resource := range c.roleAssignments {
		r[i] = types.Resource153ToLegacy(resource)
	}
	return r
}

func (c *scopedRoleAssignmentCollection) WriteText(w io.Writer, verbose bool) error {
	headers := []string{"Scope", "Name", "User", "Assigns"}
	rows := make([][]string, len(c.roleAssignments))

	for i, item := range c.roleAssignments {
		assigns := make([]string, len(item.GetSpec().GetAssignments()))
		for j, subAssignment := range item.GetSpec().GetAssignments() {
			assigns[j] = fmt.Sprintf("%s -> %s", subAssignment.GetRole(), subAssignment.GetScope())
		}

		rows[i] = []string{
			item.GetScope(),
			item.GetMetadata().GetName(),
			item.GetSpec().GetUser(),
			strings.Join(assigns, ", "),
		}
	}

	t := asciitable.MakeTable(headers, rows...)

	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func scopedRoleAssignmentHandler() Handler {
	return Handler{
		getHandler:    getScopedRoleAssignment,
		createHandler: createScopedRoleAssignment,
		deleteHandler: deleteScopedRoleAssignment,
		description:   "A scoped role assignment binds scoped role permissions to a user at a limited scope",
	}
}

func createScopedRoleAssignment(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	if opts.Force {
		return trace.BadParameter("scoped role assignment creation does not support --force")
	}

	r, err := services.UnmarshalProtoResource[*scopedaccessv1.ScopedRoleAssignment](raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	rsp, err := client.ScopedAccessServiceClient().CreateScopedRoleAssignment(ctx, &scopedaccessv1.CreateScopedRoleAssignmentRequest{
		Assignment: r,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf(
		"%v %q has been created\n",
		scopedaccess.KindScopedRoleAssignment,
		rsp.GetAssignment().GetMetadata().GetName(), // must extract from rsp since assignment names are generated server-side
	)

	return nil
}

func getScopedRoleAssignment(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	if ref.Name != "" {
		rsp, err := client.ScopedAccessServiceClient().GetScopedRoleAssignment(ctx, &scopedaccessv1.GetScopedRoleAssignmentRequest{
			Name: ref.Name,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return &scopedRoleAssignmentCollection{roleAssignments: []*scopedaccessv1.ScopedRoleAssignment{rsp.Assignment}}, nil
	}

	items, err := stream.Collect(scopedutils.RangeScopedRoleAssignments(ctx, client.ScopedAccessServiceClient(), &scopedaccessv1.ListScopedRoleAssignmentsRequest{}))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &scopedRoleAssignmentCollection{roleAssignments: items}, nil
}

func deleteScopedRoleAssignment(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if _, err := client.ScopedAccessServiceClient().DeleteScopedRoleAssignment(ctx, &scopedaccessv1.DeleteScopedRoleAssignmentRequest{
		Name: ref.Name,
	}); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf(
		"%v %q has been deleted\n",
		scopedaccess.KindScopedRoleAssignment,
		ref.Name,
	)
	return nil
}
