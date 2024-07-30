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

	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
)

type SPIFFEFederationServiceConfig struct {
	Authorizer authz.Authorizer
	Backend    services.SPIFFEFederation
	Logger     *slog.Logger
	Clock      clockwork.Clock
}

func NewSPIFFEFederationService(config SPIFFEFederationServiceConfig) *SPIFFEFederationService {
	return &SPIFFEFederationService{
		authorizer: config.Authorizer,
		backend:    config.Backend,
		logger:     config.Logger,
		clock:      config.Clock,
	}
}

type SPIFFEFederationService struct {
	machineidv1.UnimplementedSPIFFEFederationServiceServer

	authorizer authz.Authorizer
	backend    services.SPIFFEFederation
	logger     *slog.Logger
	clock      clockwork.Clock
}

func (s *SPIFFEFederationService) GetSPIFFEFederation(
	ctx context.Context, request *machineidv1.GetSPIFFEFederationRequest,
) (*machineidv1.SPIFFEFederation, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindSPIFFEFederation, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	//TODO implement me
	panic("implement me")
}

func (s *SPIFFEFederationService) ListSPIFFEFederations(
	ctx context.Context, request *machineidv1.ListSPIFFEFederationsRequest,
) (*machineidv1.ListSPIFFEFederationsResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindSPIFFEFederation, types.VerbRead, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	//TODO implement me
	panic("implement me")
}

func (s *SPIFFEFederationService) DeleteSPIFFEFederation(
	ctx context.Context, request *machineidv1.DeleteSPIFFEFederationRequest,
) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindSPIFFEFederation, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}
	//TODO implement me
	panic("implement me")
}

func (s *SPIFFEFederationService) CreateSPIFFEFederation(
	ctx context.Context, request *machineidv1.CreateSPIFFEFederationRequest,
) (*machineidv1.SPIFFEFederation, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindSPIFFEFederation, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	//TODO implement me
	panic("implement me")
}
