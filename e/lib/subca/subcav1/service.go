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

package subcav1

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"

	subcav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/subca/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/subca"
)

// CachedSubCAStorage is a subset of SubCAService containing read methods that
// may use cached results.
//
// See lib/services/local.SubCAService.
type CachedSubCAStorage interface {
	GetCertAuthorityOverride(
		context.Context, local.CertAuthorityOverrideID) (*subcav1.CertAuthorityOverride, error)
}

// SubCAStorage is the storage implementation of SubCAService.
//
// See lib/services/local.SubCAService.
type SubCAStorage interface {
	CreateCertAuthorityOverride(
		context.Context, *subcav1.CertAuthorityOverride) (*subcav1.CertAuthorityOverride, error)
}

// ServiceParams holds creation parameters for [Service].
type ServiceParams struct {
	Logger *slog.Logger

	CachedClusterNameGetter services.ClusterNameGetter
	// CachedSubCA is a cached Sub CA storage service.
	// Used by read-only RPC.
	CachedSubCA CachedSubCAStorage
	// SubCA is a non-cached Sub CA storage service.
	// Used by write RPCs.
	SubCA SubCAStorage

	Authorizer authz.Authorizer
	Emitter    apievents.Emitter
}

// Service implements the teleport.subca.v1.SubCAService RPC service.
type Service struct {
	subcav1.UnimplementedSubCAServiceServer

	logger *slog.Logger

	cachedClusterNameGetter services.ClusterNameGetter
	cachedSubCA             CachedSubCAStorage
	subCA                   SubCAStorage

	authorizer authz.Authorizer
	emitter    apievents.Emitter
}

// New creates a new [Service].
func New(p ServiceParams) (*Service, error) {
	switch {
	case p.Logger == nil:
		return nil, trace.BadParameter("param Logger required")
	case p.CachedClusterNameGetter == nil:
		return nil, trace.BadParameter("param CachedClusterNameGetter required")
	case p.CachedSubCA == nil:
		return nil, trace.BadParameter("param CachedSubCA required")
	case p.SubCA == nil:
		return nil, trace.BadParameter("param SubCA required")
	case p.Authorizer == nil:
		return nil, trace.BadParameter("param Authorizer required")
	case p.Emitter == nil:
		return nil, trace.BadParameter("param Emitter required")
	}

	return &Service{
		logger:                  p.Logger,
		cachedClusterNameGetter: p.CachedClusterNameGetter,
		cachedSubCA:             p.CachedSubCA,
		subCA:                   p.SubCA,
		authorizer:              p.Authorizer,
		emitter:                 p.Emitter,
	}, nil
}

func (s *Service) CreateCertAuthorityOverride(
	ctx context.Context,
	req *subcav1.CreateCertAuthorityOverrideRequest,
) (*subcav1.CreateCertAuthorityOverrideResponse, error) {
	switch {
	case req.CaOverride.GetMetadata().GetName() == "":
		return nil, trace.BadParameter("ca_override.metadata.name required")
	case req.CaOverride.GetSubKind() == "":
		return nil, trace.BadParameter("ca_override.sub_kind required")
	}
	if err := s.authorizeCAOverride(ctx, adminActionYes, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	parsed, err := subca.ValidateAndParseCAOverride(req.CaOverride)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Only allow overrides for the current cluster.
	cn, err := s.cachedClusterNameGetter.GetClusterName(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "read cluster name")
	}
	if cn.GetClusterName() != parsed.CAOverride.Metadata.Name {
		return nil, trace.BadParameter(
			"invalid metadata.name/clusterName: %q, only %q is allowed",
			parsed.CAOverride.Metadata.Name, cn.GetClusterName(),
		)
	}

	// TODO(codingllama): Validate against CA resource.

	// TODO(codingllama): Create CRLs.

	created, err := s.subCA.CreateCertAuthorityOverride(ctx, parsed.CAOverride)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	s.emitCAOverrideEvent(ctx,
		parsed,
		nil, // err
		events.CertAuthOverrideCreateEvent,
		events.CertAuthOverrideCreateCode,
	)

	return &subcav1.CreateCertAuthorityOverrideResponse{
		CaOverride: created,
	}, nil
}

func (s *Service) GetCertAuthorityOverride(
	ctx context.Context,
	req *subcav1.GetCertAuthorityOverrideRequest,
) (*subcav1.GetCertAuthorityOverrideResponse, error) {
	if req.CaId.GetCaType() == "" {
		return nil, trace.BadParameter("ca_id.ca_type required")
	}
	if err := s.authorizeCAOverride(ctx, adminActionNotNeeded, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	cn, err := s.cachedClusterNameGetter.GetClusterName(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "read cluster name")
	}
	caOverride, err := s.cachedSubCA.GetCertAuthorityOverride(ctx, local.CertAuthorityOverrideID{
		ClusterName: cn.GetClusterName(),
		CAType:      req.CaId.CaType,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &subcav1.GetCertAuthorityOverrideResponse{
		CaOverride: caOverride,
	}, nil
}

type adminActionMode int

const (
	adminActionYes adminActionMode = iota // stricter mode first, err on strict
	adminActionNotNeeded
)

func (s *Service) authorizeCAOverride(
	ctx context.Context,
	adminMode adminActionMode,
	verb string,
	additionalVerbs ...string,
) error {
	authzCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := authzCtx.CheckAccessToKind(
		types.KindCertAuthorityOverride, verb, additionalVerbs...); err != nil {
		return trace.Wrap(err)
	}
	if adminMode == adminActionNotNeeded {
		return nil
	}

	return trace.Wrap(authzCtx.AuthorizeAdminActionAllowReusedMFA())
}
