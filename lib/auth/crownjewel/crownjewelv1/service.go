/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package crownjewelv1

import (
	"context"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport"
	crownjewelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/crownjewel/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
)

// ServiceConfig holds configuration options for the DiscoveryConfig gRPC service.
type ServiceConfig struct {
	// Logger is the logger to use.
	Logger logrus.FieldLogger

	// Authorizer is the authorizer to use.
	Authorizer authz.Authorizer

	// Backend is the backend for storing DiscoveryConfigs.
	Backend services.CrownJewels

	// Clock is the clock.
	Clock clockwork.Clock
}

// CheckAndSetDefaults checks the ServiceConfig fields and returns an error if
// a required param is not provided.
// Authorizer, Cache and Backend are required params
func (s *ServiceConfig) CheckAndSetDefaults() error {
	if s.Authorizer == nil {
		return trace.BadParameter("authorizer is required")
	}
	if s.Backend == nil {
		return trace.BadParameter("backend is required")
	}

	if s.Logger == nil {
		s.Logger = logrus.New().WithField(teleport.ComponentKey, "crownjewel_crud_service")
	}

	if s.Clock == nil {
		s.Clock = clockwork.NewRealClock()
	}

	return nil
}

// Service implements the teleport.CrownJewel.v1.CrownJewelService RPC service.
type Service struct {
	crownjewelv1.UnimplementedCrownJewelServiceServer

	log        logrus.FieldLogger
	authorizer authz.Authorizer
	backend    services.CrownJewels
	clock      clockwork.Clock
}

// NewService returns a new CrownJewel gRPC service.
func NewService(cfg ServiceConfig) (*Service, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Service{
		log:        cfg.Logger,
		authorizer: cfg.Authorizer,
		backend:    cfg.Backend,
		clock:      cfg.Clock,
	}, nil
}

// CreateCrownJewel creates crown jewel resource.
func (s *Service) CreateCrownJewel(ctx context.Context, req *crownjewelv1.CreateCrownJewelRequest) (*crownjewelv1.CrownJewel, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindCrownJewel, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := validateCrownJewel(req.CrownJewels); err != nil {
		return nil, trace.Wrap(err)
	}

	rsp, err := s.backend.CreateCrownJewel(ctx, req.CrownJewels)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return rsp, nil
}

// ListCrownJewels returns a list of crown jewels.
func (s *Service) ListCrownJewels(ctx context.Context, req *crownjewelv1.ListCrownJewelsRequest) (*crownjewelv1.ListCrownJewelsResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindCrownJewel, types.VerbRead, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	rsp, nextToken, err := s.backend.ListCrownJewels(ctx, req.PageSize, req.PageToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &crownjewelv1.ListCrownJewelsResponse{
		CrownJewels:   rsp,
		NextPageToken: nextToken,
	}, nil
}

// UpdateCrownJewel updates crown jewel resource.
func (s *Service) UpdateCrownJewel(ctx context.Context, req *crownjewelv1.UpdateCrownJewelRequest) (*crownjewelv1.CrownJewel, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindCrownJewel, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := validateCrownJewel(req.CrownJewels); err != nil {
		return nil, trace.Wrap(err)
	}

	rsp, err := s.backend.UpdateCrownJewel(ctx, req.CrownJewels)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return rsp, nil
}

// DeleteCrownJewel deletes crown jewel resource.
func (s *Service) DeleteCrownJewel(ctx context.Context, req *crownjewelv1.DeleteCrownJewelRequest) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindCrownJewel, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.backend.DeleteCrownJewel(ctx, req.GetName()); err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}

func validateCrownJewel(jewel *crownjewelv1.CrownJewel) error {
	switch {
	case jewel == nil:
		return trace.BadParameter("crown jewel is nil")
	case jewel.Metadata == nil:
		return trace.BadParameter("crown jewel metadata is nil")
	case jewel.Metadata.Name == "":
		return trace.BadParameter("crown jewel name is empty")
	case jewel.Spec == nil:
		return trace.BadParameter("crown jewel spec is nil")
	case len(jewel.Spec.TeleportMatchers) == 0 && len(jewel.Spec.AwsMatchers) == 0:
		return trace.BadParameter("crown jewel must have at least one matcher")
	}

	if len(jewel.Spec.TeleportMatchers) > 0 {
		for _, matcher := range jewel.Spec.TeleportMatchers {
			if len(matcher.GetKinds()) == 0 {
				return trace.BadParameter("teleport matcher kinds must be set")
			}

			if matcher.Name == "" && len(matcher.GetLabels()) == 0 {
				return trace.BadParameter("teleport matcher name or labels must be set")
			}

			if len(matcher.GetLabels()) > 0 {
				for _, label := range matcher.GetLabels() {
					if label.Name == "" || len(label.Values) == 0 {
						return trace.BadParameter("teleport matcher label name or value is empty")
					}
				}
			}
		}
	}

	if len(jewel.Spec.AwsMatchers) > 0 {
		for _, matcher := range jewel.Spec.AwsMatchers {
			if len(matcher.GetTypes()) == 0 {
				return trace.BadParameter("aws matcher type must be set")
			}

			if matcher.GetArn() == "" && len(matcher.GetTags()) == 0 {
				return trace.BadParameter("aws matcher arn or tags must be set")
			}

			if len(matcher.GetTypes()) > 0 {
				for _, label := range matcher.GetTags() {
					if label.Key == "" || len(label.Values) == 0 {
						return trace.BadParameter("aws matcher tag key or value is empty")
					}
				}
			}
		}
	}

	return nil
}
