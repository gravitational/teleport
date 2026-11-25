// Copyright 2026 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
