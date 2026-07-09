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

package provider

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types/accesslist"
)

type accessListClient struct {
	client *client.Client
}

func (r accessListClient) Get(ctx context.Context, req GetResourceRequest[NameIdentifier]) (*accesslist.AccessList, error) {
	list, err := r.client.AccessListClient().GetAccessList(ctx, req.Identifier.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return list, nil
}

func (r accessListClient) Create(ctx context.Context, req *accesslist.AccessList) error {
	if _, err := r.client.AccessListClient().UpsertAccessList(ctx, req); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (r accessListClient) Upsert(ctx context.Context, req *accesslist.AccessList) error {
	if _, err := r.client.AccessListClient().UpsertAccessList(ctx, req); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (r accessListClient) Delete(ctx context.Context, req NameIdentifier) error {
	return trace.Wrap(r.client.AccessListClient().DeleteAccessList(ctx, req.Name))
}

type accessListMemberClient struct {
	client *client.Client
}

func (r accessListMemberClient) Get(ctx context.Context, req GetResourceRequest[CompositeIdentifier]) (*accesslist.AccessListMember, error) {
	list, err := r.client.AccessListClient().GetStaticAccessListMember(ctx, req.Identifier.Prefix, req.Identifier.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return list, nil
}

func (r accessListMemberClient) Create(ctx context.Context, req *accesslist.AccessListMember) error {
	if _, err := r.client.AccessListClient().UpsertStaticAccessListMember(ctx, req); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (r accessListMemberClient) Upsert(ctx context.Context, req *accesslist.AccessListMember) error {
	if _, err := r.client.AccessListClient().UpsertStaticAccessListMember(ctx, req); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (r accessListMemberClient) Delete(ctx context.Context, req CompositeIdentifier) error {
	return trace.Wrap(r.client.AccessListClient().DeleteStaticAccessListMember(ctx, req.Prefix, req.Name))
}
