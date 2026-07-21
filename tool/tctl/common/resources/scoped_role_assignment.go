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
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/scopes"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
	scopedutils "github.com/gravitational/teleport/lib/scopes/utils"
	"github.com/gravitational/teleport/lib/services"
)

type ScopedRoleAssignmentCollection struct {
	roleAssignments []*scopedaccessv1.ScopedRoleAssignment
}

func NewScopedRoleAssignmentCollection(assignments []*scopedaccessv1.ScopedRoleAssignment) Collection {
	return &ScopedRoleAssignmentCollection{
		roleAssignments: assignments,
	}
}

func (c *ScopedRoleAssignmentCollection) Resources() []types.Resource {
	r := make([]types.Resource, len(c.roleAssignments))
	for i, resource := range c.roleAssignments {
		r[i] = types.Resource153ToLegacy(resource)
	}
	return r
}

func (c *ScopedRoleAssignmentCollection) WriteText(w io.Writer, verbose bool) error {
	headers := []string{"SubKind", "ID", "User", "Assigns"}
	rows := make([][]string, len(c.roleAssignments))

	for i, item := range c.roleAssignments {
		assigns := make([]string, len(item.GetSpec().GetAssignments()))
		for j, subAssignment := range item.GetSpec().GetAssignments() {
			assigns[j] = fmt.Sprintf("%s -> %s", subAssignment.GetRole(), subAssignment.GetScope())
		}
		rows[i] = []string{
			item.GetSubKind(),
			scopes.QualifiedName{Scope: item.GetScope(), Name: item.GetMetadata().GetName()}.String(),
			item.GetSpec().GetUser(),
			strings.Join(assigns, ", "),
		}
	}

	t := asciitable.MakeTable(headers, rows...)

	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func scopedRoleAssignmentScopedHandler() ScopedHandler {
	return ScopedHandler{
		getHandler:    getScopedRoleAssignment,
		createHandler: createScopedRoleAssignment,
		updateHandler: updateScopedRoleAssignment,
		deleteHandler: deleteScopedRoleAssignment,
		description:   "A scoped role assignment binds scoped role permissions to a user at a limited scope",
	}
}

func createScopedRoleAssignment(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	r, err := services.UnmarshalProtoResource[*scopedaccessv1.ScopedRoleAssignment](raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	// use upsert when --force is set and the assignment already has a name (i.e. it was previously
	// created and the user is re-applying the same resource file). if there is no name, fall through
	// to create, which will generate one server-side.
	if opts.Force && r.GetMetadata().GetName() != "" {
		rsp, err := client.ScopedAccessServiceClient().UpsertScopedRoleAssignment(ctx, scopedaccessv1.UpsertScopedRoleAssignmentRequest_builder{
			Assignment: r,
		}.Build())
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf(
			"%v %q has been upserted\n",
			scopedaccess.KindScopedRoleAssignment,
			rsp.GetAssignment().GetMetadata().GetName(),
		)
		return nil
	}

	rsp, err := client.ScopedAccessServiceClient().CreateScopedRoleAssignment(ctx, scopedaccessv1.CreateScopedRoleAssignmentRequest_builder{
		Assignment: r,
	}.Build())
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

func updateScopedRoleAssignment(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	r, err := services.UnmarshalProtoResource[*scopedaccessv1.ScopedRoleAssignment](raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if _, err = client.ScopedAccessServiceClient().UpdateScopedRoleAssignment(ctx, scopedaccessv1.UpdateScopedRoleAssignmentRequest_builder{
		Assignment: r,
	}.Build()); err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf(
		"%v %q has been updated\n",
		scopedaccess.KindScopedRoleAssignment,
		r.GetMetadata().GetName(),
	)

	return nil
}

func getScopedRoleAssignment(ctx context.Context, client *authclient.Client, subKind string, sqn *scopes.QualifiedName, opts GetOpts) (Collection, error) {
	if sqn != nil {
		if subKind == "" {
			return nil, trace.BadParameter(
				"%s requires a sub-kind to get a specific resource, try:\n  tctl get %s/%s %s::%s",
				scopedaccess.KindScopedRoleAssignment,
				scopedaccess.KindScopedRoleAssignment, scopedaccess.SubKindDynamic,
				sqn.Scope, sqn.Name,
			)
		}
		rsp, err := client.ScopedAccessServiceClient().GetScopedRoleAssignment(ctx, scopedaccessv1.GetScopedRoleAssignmentRequest_builder{
			Name:    sqn.Name,
			SubKind: subKind,
			Scope:   sqn.Scope,
		}.Build())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &ScopedRoleAssignmentCollection{roleAssignments: []*scopedaccessv1.ScopedRoleAssignment{rsp.GetAssignment()}}, nil
	}

	items, err := stream.Collect(scopedutils.RangeScopedRoleAssignments(ctx, client.ScopedAccessServiceClient(), scopedaccessv1.ListScopedRoleAssignmentsRequest_builder{
		// exhaustive user-facing views use MODE_ALL per RFD 0229i
		ScopeFilter: scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_ALL}.Build(),
	}.Build()))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return NewScopedRoleAssignmentCollection(items), nil
}

func deleteScopedRoleAssignment(ctx context.Context, client *authclient.Client, subKind string, sqn scopes.QualifiedName) error {
	if subKind == "" {
		return trace.BadParameter(
			"%s requires a sub-kind to delete a resource, try:\n  tctl rm %s/%s %s::%s",
			scopedaccess.KindScopedRoleAssignment,
			scopedaccess.KindScopedRoleAssignment, scopedaccess.SubKindDynamic,
			sqn.Scope, sqn.Name,
		)
	}
	if subKind == scopedaccess.SubKindMaterialized {
		return trace.BadParameter("%s scoped_role_assignments are derived from access lists and cannot be deleted directly", scopedaccess.SubKindMaterialized)
	}

	if _, err := client.ScopedAccessServiceClient().DeleteScopedRoleAssignment(ctx, scopedaccessv1.DeleteScopedRoleAssignmentRequest_builder{
		Name:    sqn.Name,
		SubKind: subKind,
		Scope:   sqn.Scope,
	}.Build()); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf(
		"%v %q has been deleted\n",
		scopedaccess.KindScopedRoleAssignment,
		sqn.Name,
	)
	return nil
}
