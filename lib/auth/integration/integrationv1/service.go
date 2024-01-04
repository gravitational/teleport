/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package integrationv1

import (
	"context"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/emptypb"

	integrationpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/keystore"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
)

// CAGetter describes the required methods to sign a JWT to be used for AWS OIDC Integration.
type CAGetter interface {
	// GetDomainName returns local auth domain of the current auth server
	GetDomainName() (string, error)

	// GetCertAuthority returns certificate authority by given id. Parameter loadSigningKeys
	// controls if signing keys are loaded
	GetCertAuthority(ctx context.Context, id types.CertAuthID, loadSigningKeys bool) (types.CertAuthority, error)

	// GetKeyStore returns the KeyStore used by the auth server
	GetKeyStore() *keystore.Manager
}

// ServiceConfig holds configuration options for
// the Integration gRPC service.
type ServiceConfig struct {
	Authorizer authz.Authorizer
	Cache      services.IntegrationsGetter
	Backend    services.Integrations
	CAGetter   CAGetter
	Logger     *logrus.Entry
	Clock      clockwork.Clock
}

// CheckAndSetDefaults checks the ServiceConfig fields and returns an error if
// a required param is not provided.
// Authorizer, Cache and Backend are required params
func (s *ServiceConfig) CheckAndSetDefaults() error {
	if s.Cache == nil {
		return trace.BadParameter("cache is required")
	}

	if s.Backend == nil {
		return trace.BadParameter("backend is required")
	}

	if s.Authorizer == nil {
		return trace.BadParameter("authorizer is required")
	}

	if s.CAGetter == nil {
		return trace.BadParameter("ca getter is required")
	}

	if s.Logger == nil {
		s.Logger = logrus.WithField(trace.Component, "integrations.service")
	}

	if s.Clock == nil {
		s.Clock = clockwork.NewRealClock()
	}

	return nil
}

// Service implements the teleport.integration.v1.IntegrationService RPC service.
type Service struct {
	integrationpb.UnimplementedIntegrationServiceServer
	authorizer authz.Authorizer
	cache      services.IntegrationsGetter
	backend    services.Integrations
	caGetter   CAGetter
	logger     *logrus.Entry
	clock      clockwork.Clock
}

// NewService returns a new Integrations gRPC service.
func NewService(cfg *ServiceConfig) (*Service, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Service{
		logger:     cfg.Logger,
		authorizer: cfg.Authorizer,
		cache:      cfg.Cache,
		backend:    cfg.Backend,
		caGetter:   cfg.CAGetter,
		clock:      cfg.Clock,
	}, nil
}

var _ integrationpb.IntegrationServiceServer = (*Service)(nil)

// ListIntegrations returns a paginated list of all Integration resources.
func (s *Service) ListIntegrations(ctx context.Context, req *integrationpb.ListIntegrationsRequest) (*integrationpb.ListIntegrationsResponse, error) {
	_, err := authz.AuthorizeWithVerbs(ctx, s.logger, s.authorizer, true, types.KindIntegration, types.VerbRead, types.VerbList)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	results, nextKey, err := s.cache.ListIntegrations(ctx, int(req.GetLimit()), req.GetNextKey())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	igs := make([]*types.IntegrationV1, len(results))
	for i, r := range results {
		v1, ok := r.(*types.IntegrationV1)
		if !ok {
			return nil, trace.BadParameter("unexpected Integration type %T", r)
		}
		igs[i] = v1
	}

	return &integrationpb.ListIntegrationsResponse{
		Integrations: igs,
		NextKey:      nextKey,
	}, nil
}

// GetIntegration returns the specified Integration resource.
func (s *Service) GetIntegration(ctx context.Context, req *integrationpb.GetIntegrationRequest) (*types.IntegrationV1, error) {
	_, err := authz.AuthorizeWithVerbs(ctx, s.logger, s.authorizer, true, types.KindIntegration, types.VerbRead)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	integration, err := s.cache.GetIntegration(ctx, req.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	igV1, ok := integration.(*types.IntegrationV1)
	if !ok {
		return nil, trace.BadParameter("unexpected Integration type %T", integration)
	}

	return igV1, nil
}

// CreateIntegration creates a new Okta import rule resource.
func (s *Service) CreateIntegration(ctx context.Context, req *integrationpb.CreateIntegrationRequest) (*types.IntegrationV1, error) {
	_, err := authz.AuthorizeWithVerbs(ctx, s.logger, s.authorizer, true, types.KindIntegration, types.VerbCreate)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ig, err := s.backend.CreateIntegration(ctx, req.GetIntegration())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	igV1, ok := ig.(*types.IntegrationV1)
	if !ok {
		return nil, trace.BadParameter("unexpected Integration type %T", ig)
	}

	return igV1, nil
}

// UpdateIntegration updates an existing Okta import rule resource.
func (s *Service) UpdateIntegration(ctx context.Context, req *integrationpb.UpdateIntegrationRequest) (*types.IntegrationV1, error) {
	_, err := authz.AuthorizeWithVerbs(ctx, s.logger, s.authorizer, true, types.KindIntegration, types.VerbUpdate)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ig, err := s.backend.UpdateIntegration(ctx, req.GetIntegration())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	igV1, ok := ig.(*types.IntegrationV1)
	if !ok {
		return nil, trace.BadParameter("unexpected Integration type %T", ig)
	}

	return igV1, nil
}

// DeleteIntegration removes the specified Integration resource.
func (s *Service) DeleteIntegration(ctx context.Context, req *integrationpb.DeleteIntegrationRequest) (*emptypb.Empty, error) {
	_, err := authz.AuthorizeWithVerbs(ctx, s.logger, s.authorizer, true, types.KindIntegration, types.VerbDelete)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.backend.DeleteIntegration(ctx, req.GetName()); err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}

// DeleteAllIntegrations removes all Integration resources.
func (s *Service) DeleteAllIntegrations(ctx context.Context, _ *integrationpb.DeleteAllIntegrationsRequest) (*emptypb.Empty, error) {
	_, err := authz.AuthorizeWithVerbs(ctx, s.logger, s.authorizer, true, types.KindIntegration, types.VerbDelete)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.backend.DeleteAllIntegrations(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}
