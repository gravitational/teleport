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

type scopedRoleCollection struct {
	roles []*scopedaccessv1.ScopedRole
}

func NewScopedRoleCollection(roles []*scopedaccessv1.ScopedRole) Collection {
	return &scopedRoleCollection{
		roles: roles,
	}
}

func (c *scopedRoleCollection) Resources() []types.Resource {
	r := make([]types.Resource, len(c.roles))
	for i, resource := range c.roles {
		r[i] = types.Resource153ToLegacy(resource)
	}
	return r
}

func (c *scopedRoleCollection) WriteText(w io.Writer, verbose bool) error {
	headers := []string{"Scope", "Name"}
	rows := make([][]string, len(c.roles))
	for i, item := range c.roles {
		rows[i] = []string{
			item.GetScope(),
			item.GetMetadata().GetName(),
		}
	}

	t := asciitable.MakeTable(headers, rows...)

	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func scopedRoleHandler() Handler {
	return Handler{
		getHandler:    getScopedRole,
		createHandler: createScopedRole,
		updateHandler: updateScopedRole,
		deleteHandler: deleteScopedRole,
		description:   "A set of permissions that can be granted to users at a limited scope",
	}
}

func createScopedRole(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	if opts.Force {
		return trace.BadParameter("scoped role creation does not support --force")
	}

	r, err := services.UnmarshalProtoResource[*scopedaccessv1.ScopedRole](raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if _, err := client.ScopedAccessServiceClient().CreateScopedRole(ctx, &scopedaccessv1.CreateScopedRoleRequest{
		Role: r,
	}); err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf(
		"%v %q has been created\n",
		scopedaccess.KindScopedRole,
		r.GetMetadata().GetName(),
	)

	return nil
}

func updateScopedRole(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	r, err := services.UnmarshalProtoResource[*scopedaccessv1.ScopedRole](raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if _, err = client.ScopedAccessServiceClient().UpdateScopedRole(ctx, &scopedaccessv1.UpdateScopedRoleRequest{
		Role: r,
	}); err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf(
		"%v %q has been updated\n",
		scopedaccess.KindScopedRole,
		r.GetMetadata().GetName(),
	)

	return nil
}

func getScopedRole(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	if ref.Name != "" {
		rsp, err := client.ScopedAccessServiceClient().GetScopedRole(ctx, &scopedaccessv1.GetScopedRoleRequest{
			Name: ref.Name,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return &scopedRoleCollection{roles: []*scopedaccessv1.ScopedRole{rsp.Role}}, nil
	}

	items, err := stream.Collect(scopedutils.RangeScopedRoles(ctx, client.ScopedAccessServiceClient(), &scopedaccessv1.ListScopedRolesRequest{}))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &scopedRoleCollection{roles: items}, nil
}

func deleteScopedRole(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if _, err := client.ScopedAccessServiceClient().DeleteScopedRole(ctx, &scopedaccessv1.DeleteScopedRoleRequest{
		Name: ref.Name,
	}); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf(
		"%v %q has been deleted\n",
		scopedaccess.KindScopedRole,
		ref.Name,
	)
	return nil
}
