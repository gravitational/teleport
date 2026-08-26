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

package local

import (
	"context"

	"github.com/gravitational/trace"

	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	apitypes "github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

const workloadIdentityX509IssuerOverridePrefix = "workload_identity_x509_issuer_override"

func NewWorkloadIdentityX509OverridesService(b backend.Backend) (*WorkloadIdentityX509OverridesService, error) {
	// issuer overrides can be a bit bulky in terms of size, so we deviate from
	// the default of 1000
	const pageLimit = 100

	issuer, err := generic.NewServiceWrapper(generic.ServiceConfig[*workloadidentityv1pb.X509IssuerOverride]{
		Backend:       b,
		PageLimit:     pageLimit,
		ResourceKind:  apitypes.KindWorkloadIdentityX509IssuerOverride,
		BackendPrefix: backend.NewKey(workloadIdentityX509IssuerOverridePrefix),
		MarshalFunc:   services.MarshalProtoResource[*workloadidentityv1pb.X509IssuerOverride],
		UnmarshalFunc: services.UnmarshalProtoResource[*workloadidentityv1pb.X509IssuerOverride],
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &WorkloadIdentityX509OverridesService{
		issuer: issuer,
	}, nil
}

type WorkloadIdentityX509OverridesService struct {
	issuer *generic.ServiceWrapper[*workloadidentityv1pb.X509IssuerOverride]
}

var _ services.WorkloadIdentityX509Overrides = (*WorkloadIdentityX509OverridesService)(nil)

// GetX509IssuerOverride implements [services.WorkloadIdentityX509Overrides].
func (s *WorkloadIdentityX509OverridesService) GetX509IssuerOverride(ctx context.Context, name string) (*workloadidentityv1pb.X509IssuerOverride, error) {
	return s.issuer.GetResource(ctx, name)
}

// ListX509IssuerOverrides implements [services.WorkloadIdentityX509Overrides].
func (s *WorkloadIdentityX509OverridesService) ListX509IssuerOverrides(ctx context.Context, pageSize int, pageToken string) (_ []*workloadidentityv1pb.X509IssuerOverride, nextPageToken string, _ error) {
	return s.issuer.ListResources(ctx, pageSize, pageToken)
}

// CreateX509IssuerOverride implements [services.WorkloadIdentityX509Overrides].
func (s *WorkloadIdentityX509OverridesService) CreateX509IssuerOverride(ctx context.Context, resource *workloadidentityv1pb.X509IssuerOverride) (*workloadidentityv1pb.X509IssuerOverride, error) {
	return s.issuer.CreateResource(ctx, resource)
}

// UpdateX509IssuerOverride implements [services.WorkloadIdentityX509Overrides].
func (s *WorkloadIdentityX509OverridesService) UpdateX509IssuerOverride(ctx context.Context, resource *workloadidentityv1pb.X509IssuerOverride) (*workloadidentityv1pb.X509IssuerOverride, error) {
	return s.issuer.ConditionalUpdateResource(ctx, resource)
}

// UpsertX509IssuerOverride implements [services.WorkloadIdentityX509Overrides].
func (s *WorkloadIdentityX509OverridesService) UpsertX509IssuerOverride(ctx context.Context, resource *workloadidentityv1pb.X509IssuerOverride) (*workloadidentityv1pb.X509IssuerOverride, error) {
	return s.issuer.UpsertResource(ctx, resource)
}

// DeleteX509IssuerOverride implements [services.WorkloadIdentityX509Overrides].
func (s *WorkloadIdentityX509OverridesService) DeleteX509IssuerOverride(ctx context.Context, name string) error {
	return s.issuer.DeleteResource(ctx, name)
}
