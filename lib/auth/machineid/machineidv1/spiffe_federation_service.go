// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package machineidv1

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
)

// SPIFFEFederationServiceConfig holds configuration options for
// NewSPIFFEFederationService
type SPIFFEFederationServiceConfig struct {
	Authorizer authz.Authorizer
	Backend    services.SPIFFEFederation
	Logger     *slog.Logger
	Clock      clockwork.Clock
	Emitter    apievents.Emitter
}

// NewSPIFFEFederationService returns a new instance of the SPIFFEFederationService.
func NewSPIFFEFederationService(
	cfg SPIFFEFederationServiceConfig,
) (*SPIFFEFederationService, error) {
	switch {
	case cfg.Backend == nil:
		return nil, trace.BadParameter("backend service is required")
	case cfg.Authorizer == nil:
		return nil, trace.BadParameter("authorizer is required")
	case cfg.Emitter == nil:
		return nil, trace.BadParameter("emitter is required")
	}

	if cfg.Logger == nil {
		cfg.Logger = slog.With(teleport.ComponentKey, "spiffe_federation.service")
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}

	return &SPIFFEFederationService{
		authorizer: cfg.Authorizer,
		backend:    cfg.Backend,
		logger:     cfg.Logger,
		clock:      cfg.Clock,
	}, nil
}

// SPIFFEFederationService is an implementation of
// teleport.machineid.v1.SPIFFEFederationService
type SPIFFEFederationService struct {
	machineidv1.UnimplementedSPIFFEFederationServiceServer

	authorizer authz.Authorizer
	backend    services.SPIFFEFederation
	logger     *slog.Logger
	clock      clockwork.Clock
	emitter    apievents.Emitter
}

// GetSPIFFEFederation returns a SPIFFE Federation by name.
// Implements teleport.machineid.v1.SPIFFEFederationService/GetSPIFFEFederation
func (s *SPIFFEFederationService) GetSPIFFEFederation(
	ctx context.Context, req *machineidv1.GetSPIFFEFederationRequest,
) (*machineidv1.SPIFFEFederation, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindSPIFFEFederation, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	if req.Name == "" {
		return nil, trace.BadParameter("name: must be non-empty")
	}

	// TODO(noah): Use cache...
	federation, err := s.backend.GetSPIFFEFederation(ctx, req.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return federation, nil
}

// ListSPIFFEFederations returns a list of SPIFFE Federations. It follows the
// Google API design guidelines for list pagination.
// Implements teleport.machineid.v1.SPIFFEFederationService/ListSPIFFEFederations
func (s *SPIFFEFederationService) ListSPIFFEFederations(
	ctx context.Context, req *machineidv1.ListSPIFFEFederationsRequest,
) (*machineidv1.ListSPIFFEFederationsResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindSPIFFEFederation, types.VerbRead, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO: Use cache...
	federations, nextToken, err := s.backend.ListSPIFFEFederations(
		ctx,
		int(req.PageSize),
		req.PageToken,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &machineidv1.ListSPIFFEFederationsResponse{
		SpiffeFederations: federations,
		NextPageToken:     nextToken,
	}, nil
}

// DeleteSPIFFEFederation deletes a SPIFFE Federation by name.
// Implements teleport.machineid.v1.SPIFFEFederationService/DeleteSPIFFEFederation
func (s *SPIFFEFederationService) DeleteSPIFFEFederation(
	ctx context.Context, req *machineidv1.DeleteSPIFFEFederationRequest,
) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindSPIFFEFederation, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	if req.Name == "" {
		return nil, trace.BadParameter("name: must be non-empty")
	}

	if err := s.backend.DeleteSPIFFEFederation(ctx, req.Name); err != nil {
		return nil, trace.Wrap(err)
	}
	// TODO: audit log
	return &emptypb.Empty{}, nil
}

// CreateSPIFFEFederation creates a new SPIFFE Federation.
// Implements teleport.machineid.v1.SPIFFEFederationService/CreateSPIFFEFederation
func (s *SPIFFEFederationService) CreateSPIFFEFederation(
	ctx context.Context, req *machineidv1.CreateSPIFFEFederationRequest,
) (*machineidv1.SPIFFEFederation, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindSPIFFEFederation, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	created, err := s.backend.CreateSPIFFEFederation(ctx, req.SpiffeFederation)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// TODO: audit log
	return created, nil
}
