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
	"time"

	"github.com/gravitational/trace"

	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/accesslists"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/services"
)

type accessListCollection struct {
	accessLists []*accesslist.AccessList
}

func (c *accessListCollection) Resources() []types.Resource {
	r := make([]types.Resource, len(c.accessLists))
	for i, resource := range c.accessLists {
		r[i] = resource
	}
	return r
}

func (c *accessListCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "Title", "Review Frequency", "Next Audit Date"})
	for _, al := range c.accessLists {
		t.AddRow([]string{
			accesslists.ScopeQualifiedName(al).String(),
			al.Spec.Title,
			al.Spec.Audit.Recurrence.Frequency.String(),
			al.Spec.Audit.NextAuditDate.Format(time.RFC822),
		})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func accessListHandler() Handler {
	return Handler{
		getHandler:    getUnscopedAccessList,
		createHandler: createAccessList,
		deleteHandler: deleteUnscopedAccessList,
		singleton:     false,
		mfaRequired:   true,
		description:   "Used to grant roles or traits to users or other lists. Part of Identity Governance.",
	}
}

func accessListScopedHandler() ScopedHandler {
	return ScopedHandler{
		getHandler:    getAccessList,
		createHandler: createAccessList,
		deleteHandler: deleteAccessList,
		mfaRequired:   true,
		description:   "Used to grant roles or traits to users or other lists. Part of Identity Governance.",
	}
}

// getAccessList implements `tctl get accesslist /scope::my-list` command.
func getAccessList(ctx context.Context, client *authclient.Client, subKind string, sqn *scopes.QualifiedName, opts GetOpts) (Collection, error) {
	if subKind != "" {
		return nil, rejectSubKind(types.KindAccessList, subKind)
	}

	if sqn != nil {
		resource, err := client.AccessListClient().GetAccessListV2(ctx, accesslistv1.GetAccessListRequest_builder{
			Scope: sqn.Scope,
			Name:  sqn.Name,
		}.Build())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &accessListCollection{accessLists: []*accesslist.AccessList{resource}}, nil
	}

	accessLists, err := stream.Collect(
		clientutils.Resources(ctx, func(ctx context.Context, size int, token string) ([]*accesslist.AccessList, string, error) {
			return client.AccessListClient().ListAccessListsV2(ctx,
				accesslistv1.ListAccessListsV2Request_builder{
					PageSize:  int32(size),
					PageToken: token,
					ScopeFilter: scopesv1.Filter_builder{
						Mode: scopesv1.Mode_MODE_ALL,
					}.Build(),
				}.Build())
		}),
	)

	return &accessListCollection{accessLists: accessLists}, trace.Wrap(err)
}

// getUnscopedAccessList implements `tctl get accesslist/my-list` command.
func getUnscopedAccessList(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	if ref.Name != "" {
		return getAccessList(ctx, client, ref.SubKind, &scopes.QualifiedName{Name: ref.Name}, opts)
	}
	return getAccessList(ctx, client, ref.SubKind, nil, opts)
}

// createAccessList implements `tctl create accesslist/my-list` command.
func createAccessList(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	accessList, err := services.UnmarshalAccessList(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = client.AccessListClient().GetAccessListV2(ctx, accesslistv1.GetAccessListRequest_builder{
		Scope: accessList.GetScope(),
		Name:  accessList.GetName(),
	}.Build())
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	exists := (err == nil)

	if exists && !opts.Force {
		return trace.AlreadyExists("Access list %q already exists", accessList.GetName())
	}

	if _, err := client.AccessListClient().UpsertAccessList(ctx, accessList); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("Access list %q has been %s\n", accessList.GetName(), upsertVerb(exists, opts.Force))

	return nil

}

// deleteAccessList implements `tctl rm accesslist /scope::my-list` command.
func deleteAccessList(ctx context.Context, client *authclient.Client, subKind string, sqn scopes.QualifiedName) error {
	if subKind != "" {
		return rejectSubKind(types.KindAccessList, subKind)
	}

	if err := client.AccessListClient().DeleteAccessListV2(ctx, accesslistv1.DeleteAccessListRequest_builder{
		Scope: sqn.Scope,
		Name:  sqn.Name,
	}.Build()); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("Access list %q has been deleted\n", sqn.String())
	return nil
}

func deleteUnscopedAccessList(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	return deleteAccessList(ctx, client, ref.SubKind, scopes.QualifiedName{Name: ref.Name})
}
