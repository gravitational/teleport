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

// NewAccessListMemberClient returns an access list member client.
func NewAccessListMemberClient(c *client.Client) AccessListMemberClient {
	return AccessListMemberClient{client: c}
}

// AccessListMemberClient manages access list member resources.
type AccessListMemberClient struct {
	client *client.Client
}

// Get reads an access list member by name.
func (r AccessListMemberClient) Get(ctx context.Context, id tfdriver.ScopeQualifiedCompositeIdentifier) (*accesslist.AccessListMember, error) {
	member, err := r.client.AccessListClient().GetStaticAccessListMemberV2(ctx, accesslistv1.GetStaticAccessListMemberRequest_builder{
		AccessListScope: id.Prefix.Scope,
		AccessList:      id.Prefix.Name,
		MemberScope:     id.Name.Scope,
		MemberName:      id.Name.Name,
	}.Build())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return member, nil
}

// Create creates an access list member.
func (r AccessListMemberClient) Create(ctx context.Context, member *accesslist.AccessListMember) error {
	if _, err := r.client.AccessListClient().UpsertStaticAccessListMember(ctx, member); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Upsert updates an access list member.
func (r AccessListMemberClient) Upsert(ctx context.Context, member *accesslist.AccessListMember) error {
	if _, err := r.client.AccessListClient().UpsertStaticAccessListMember(ctx, member); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Delete deletes an access list member by name.
func (r AccessListMemberClient) Delete(ctx context.Context, id tfdriver.ScopeQualifiedCompositeIdentifier) error {
	return trace.Wrap(r.client.AccessListClient().DeleteStaticAccessListMemberV2(ctx, accesslistv1.DeleteStaticAccessListMemberRequest_builder{
		AccessListScope: id.Prefix.Scope,
		AccessList:      id.Prefix.Name,
		MemberScope:     id.Name.Scope,
		MemberName:      id.Name.Name,
	}.Build()))
}
