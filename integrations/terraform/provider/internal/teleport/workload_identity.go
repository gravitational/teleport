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
	workloadidentityv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
)

// NewWorkloadIdentityClient returns a workload identity client.
func NewWorkloadIdentityClient(c *client.Client) WorkloadIdentityClient {
	return WorkloadIdentityClient{client: c}
}

// WorkloadIdentityClient manages workload identity resources.
type WorkloadIdentityClient struct {
	client *client.Client
}

// Get reads a workload identity by name.
func (r WorkloadIdentityClient) Get(ctx context.Context, id tfdriver.ScopeQualifiedNameIdentifier) (*workloadidentityv1.WorkloadIdentity, error) {
	resp, err := r.client.WorkloadIdentityResourceServiceClient().GetWorkloadIdentity(ctx, &workloadidentityv1.GetWorkloadIdentityRequest{
		Name:  id.Name,
		Scope: id.Scope,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return resp, nil
}

// Create creates a workload identity.
func (r WorkloadIdentityClient) Create(ctx context.Context, workloadIdentity *workloadidentityv1.WorkloadIdentity) error {
	if _, err := r.client.CreateWorkloadIdentity(ctx, workloadIdentity); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Upsert updates a workload identity.
func (r WorkloadIdentityClient) Upsert(ctx context.Context, workloadIdentity *workloadidentityv1.WorkloadIdentity) error {
	if _, err := r.client.UpsertWorkloadIdentity(ctx, workloadIdentity); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Delete deletes a workload identity by name.
func (r WorkloadIdentityClient) Delete(ctx context.Context, id tfdriver.ScopeQualifiedNameIdentifier) error {
	_, err := r.client.WorkloadIdentityResourceServiceClient().DeleteWorkloadIdentity(ctx, &workloadidentityv1.DeleteWorkloadIdentityRequest{
		Name:  id.Name,
		Scope: id.Scope,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}
