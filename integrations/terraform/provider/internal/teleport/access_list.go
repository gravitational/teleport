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
