// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package teleport

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client"
	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	"github.com/gravitational/teleport/api/types/accesslist"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
)

// NewAccessListClient returns an access list client.
func NewAccessListClient(c *client.Client) AccessListClient {
	return AccessListClient{client: c}
}

// AccessListClient manages access list resources.
type AccessListClient struct {
	client *client.Client
}

// Get reads an access list by name.
func (r AccessListClient) Get(ctx context.Context, id tfdriver.ScopeQualifiedNameIdentifier) (*accesslist.AccessList, error) {
	al, err := r.client.AccessListClient().GetAccessListV2(ctx, &accesslistv1.GetAccessListRequest{
		Name:  id.Name,
		Scope: id.Scope,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return al, nil
}

// Create creates an access list.
func (r AccessListClient) Create(ctx context.Context, al *accesslist.AccessList) error {
	if _, err := r.client.AccessListClient().UpsertAccessList(ctx, al); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// PrepareUpdate preserves server-computed fields that should not be reset on update.
func (r AccessListClient) PrepareUpdate(before, after *accesslist.AccessList) error {
	after.Spec.Audit.NextAuditDate = before.Spec.Audit.NextAuditDate
	return nil
}

// Upsert updates an access list.
func (r AccessListClient) Upsert(ctx context.Context, al *accesslist.AccessList) error {
	if _, err := r.client.AccessListClient().UpsertAccessList(ctx, al); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Delete deletes an access list by name.
func (r AccessListClient) Delete(ctx context.Context, id tfdriver.ScopeQualifiedNameIdentifier) error {
	return trace.Wrap(r.client.AccessListClient().DeleteAccessListV2(ctx, &accesslistv1.DeleteAccessListRequest{
		Name:  id.Name,
		Scope: id.Scope,
	}))
}
