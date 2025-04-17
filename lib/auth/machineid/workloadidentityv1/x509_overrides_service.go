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

package workloadidentityv1

import (
	"context"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/emptypb"

	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	apitypes "github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
)

type X509OverridesServiceConfig struct {
	Authorizer authz.Authorizer
	Storage    services.WorkloadIdentityX509Overrides
	Emitter    apievents.Emitter

	ClusterName string
}

// NewX509OverridesService returns an implementation of
// [workloadidentityv1pb.X509OverridesServiceServer] that only allows reading
// and deleting override resources, suitable for checking and cleaning things up
// after downgrading from a licensed version of Teleport. A matching
// fully-featured implementation can be found in
// e/lib/auth/machineid/workloadidentityv1 .
func NewX509OverridesService(cfg X509OverridesServiceConfig) (*X509OverridesService, error) {
	if cfg.Authorizer == nil {
		return nil, trace.BadParameter("authorizer is required")
	}
	if cfg.Storage == nil {
		return nil, trace.BadParameter("storage is required")
	}
	if cfg.Emitter == nil {
		return nil, trace.BadParameter("emitter is required")
	}

	if cfg.ClusterName == "" {
		return nil, trace.BadParameter("cluster name is required")
	}

	return &X509OverridesService{
		authorizer: cfg.Authorizer,
		storage:    cfg.Storage,
		emitter:    cfg.Emitter,

		clusterName: cfg.ClusterName,
	}, nil
}

// X509OverridesService implements the non-enterprise version of
// [workloadidentityv1pb.X509OverridesServiceServer], only allowing reading,
// listing and deleting any stored state.
type X509OverridesService struct {
	workloadidentityv1pb.UnsafeX509OverridesServiceServer

	authorizer authz.Authorizer
	storage    services.WorkloadIdentityX509Overrides
	emitter    apievents.Emitter

	clusterName string
}

var _ workloadidentityv1pb.X509OverridesServiceServer = (*X509OverridesService)(nil)

func (s *X509OverridesService) authorizeAccessToKind(ctx context.Context, kind string, verb string, additionalVerbs ...string) error {
	authzCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	return authzCtx.CheckAccessToKind(kind, verb, additionalVerbs...)
}

func (s *X509OverridesService) authorizeAccessToKindAdminReusedMFA(ctx context.Context, kind string, verb string, additionalVerbs ...string) error {
	authzCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := authzCtx.CheckAccessToKind(kind, verb, additionalVerbs...); err != nil {
		return trace.Wrap(err)
	}
	if err := authzCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (s *X509OverridesService) requireEnterprise() error {
	return trace.AccessDenied("SPIFFE X.509 issuer overrides are only available with an enterprise license")
}

// SignX509IssuerCSR implements [workloadidentityv1pb.X509OverridesServiceServer].
func (s *X509OverridesService) SignX509IssuerCSR(ctx context.Context, req *workloadidentityv1pb.SignX509IssuerCSRRequest) (*workloadidentityv1pb.SignX509IssuerCSRResponse, error) {
	return nil, s.requireEnterprise()
}

// GetX509IssuerOverride implements [workloadidentityv1pb.X509OverridesServiceServer].
func (s *X509OverridesService) GetX509IssuerOverride(ctx context.Context, req *workloadidentityv1pb.GetX509IssuerOverrideRequest) (*workloadidentityv1pb.X509IssuerOverride, error) {
	if err := s.authorizeAccessToKind(ctx, apitypes.KindWorkloadIdentityX509IssuerOverride, apitypes.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	return s.storage.GetX509IssuerOverride(ctx, req.GetName())
}

// ListX509IssuerOverrides implements [workloadidentityv1pb.X509OverridesServiceServer].
func (s *X509OverridesService) ListX509IssuerOverrides(ctx context.Context, req *workloadidentityv1pb.ListX509IssuerOverridesRequest) (*workloadidentityv1pb.ListX509IssuerOverridesResponse, error) {
	if err := s.authorizeAccessToKind(ctx, apitypes.KindWorkloadIdentityX509IssuerOverride, apitypes.VerbList, apitypes.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	overrides, nextPageToken, err := s.storage.ListX509IssuerOverrides(ctx, int(req.GetPageSize()), req.GetPageToken())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &workloadidentityv1pb.ListX509IssuerOverridesResponse{
		X509IssuerOverrides: overrides,
		NextPageToken:       nextPageToken,
	}, nil
}

// CreateX509IssuerOverride implements [workloadidentityv1pb.X509OverridesServiceServer].
func (s *X509OverridesService) CreateX509IssuerOverride(ctx context.Context, req *workloadidentityv1pb.CreateX509IssuerOverrideRequest) (*workloadidentityv1pb.X509IssuerOverride, error) {
	return nil, s.requireEnterprise()
}

// UpdateX509IssuerOverride implements [workloadidentityv1pb.X509OverridesServiceServer].
func (s *X509OverridesService) UpdateX509IssuerOverride(ctx context.Context, req *workloadidentityv1pb.UpdateX509IssuerOverrideRequest) (*workloadidentityv1pb.X509IssuerOverride, error) {
	return nil, s.requireEnterprise()
}

// UpsertX509IssuerOverride implements [workloadidentityv1pb.X509OverridesServiceServer].
func (s *X509OverridesService) UpsertX509IssuerOverride(ctx context.Context, req *workloadidentityv1pb.UpsertX509IssuerOverrideRequest) (*workloadidentityv1pb.X509IssuerOverride, error) {
	return nil, s.requireEnterprise()
}

// DeleteX509IssuerOverride implements [workloadidentityv1pb.X509OverridesServiceServer].
func (s *X509OverridesService) DeleteX509IssuerOverride(ctx context.Context, req *workloadidentityv1pb.DeleteX509IssuerOverrideRequest) (*emptypb.Empty, error) {
	if err := s.authorizeAccessToKindAdminReusedMFA(ctx, apitypes.KindWorkloadIdentityX509IssuerOverride, apitypes.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.storage.DeleteX509IssuerOverride(ctx, req.GetName()); err != nil {
		return nil, trace.Wrap(err)
	}

	s.emitter.EmitAuditEvent(ctx, &apievents.WorkloadIdentityX509IssuerOverrideDelete{
		Metadata: apievents.Metadata{
			Type: events.WorkloadIdentityX509IssuerOverrideDeleteEvent,
			Code: events.WorkloadIdentityX509IssuerOverrideDeleteCode,
		},
		UserMetadata:       authz.ClientUserMetadata(ctx),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: req.GetName(),
		},
	})

	return &emptypb.Empty{}, nil
}
