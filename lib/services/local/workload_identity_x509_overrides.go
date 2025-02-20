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
	"encoding/hex"

	"github.com/gravitational/trace"

	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

const x509IssuerOverridePrefix = "workload_identity_x509_issuer_override"

func NewWorkloadIdentityX509OverridesService(b backend.Backend) (*WorkloadIdentityX509OverridesService, error) {
	svc := &WorkloadIdentityX509OverridesService{}

	issuer, err := generic.NewServiceWrapper(generic.ServiceWrapperConfig[*workloadidentityv1pb.X509IssuerOverride]{
		Backend: b,
		// issuer overrides can be a bit bulky in terms of size
		PageLimit:     100,
		ResourceKind:  types.KindWorkloadIdentityX509IssuerOverride,
		BackendPrefix: backend.NewKey(x509IssuerOverridePrefix),
		MarshalFunc:   services.MarshalProtoResource[*workloadidentityv1pb.X509IssuerOverride],
		UnmarshalFunc: services.UnmarshalProtoResource[*workloadidentityv1pb.X509IssuerOverride],
		KeyFunc: func(r *workloadidentityv1pb.X509IssuerOverride) string {
			return svc.keyFunc(r.GetMetadata().GetName())
		},
		KeyForNameFunc: svc.keyFunc,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	svc.issuer = issuer

	return svc, nil
}

type WorkloadIdentityX509OverridesService struct {
	issuer *generic.ServiceWrapper[*workloadidentityv1pb.X509IssuerOverride]
}

func (*WorkloadIdentityX509OverridesService) keyFunc(name string) string {
	return hex.EncodeToString([]byte(name))
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
