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

package decisionv1

import (
	"cmp"
	"context"
	"log/slog"

	"github.com/gravitational/trace"

	decisionpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/decision/v1alpha1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/decision"
)

// ServiceConfig holds creation parameters for [Service].
type ServiceConfig struct {
	DecisionService *decision.Service
	// Authorizer used by the service.
	Authorizer authz.Authorizer
	Logger     *slog.Logger
}

// Service implements the teleport.decision.v1alpha1.DecisionService gRPC API.
type Service struct {
	decisionpb.UnimplementedDecisionServiceServer
	pdp        *decision.Service
	authorizer authz.Authorizer
	logger     *slog.Logger
}

// NewService creates a new [Service] instance.
func NewService(cfg ServiceConfig) (*Service, error) {
	if cfg.DecisionService == nil {
		return nil, trace.BadParameter("param DecisionService required")
	}

	if cfg.Authorizer == nil {
		return nil, trace.BadParameter("param Authorizer required")
	}

	return &Service{
		pdp:        cfg.DecisionService,
		authorizer: cfg.Authorizer,
		logger:     cmp.Or(cfg.Logger, slog.Default()),
	}, nil
}

func (s *Service) EvaluateSSHAccess(ctx context.Context, req *decisionpb.EvaluateSSHAccessRequest) (*decisionpb.EvaluateSSHAccessResponse, error) {
	authzContext, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.HasBuiltinRole(*authzContext, string(types.RoleAdmin)) {
		s.logger.WarnContext(ctx, "user does not have permission to evaluate SSH access", "user", authzContext.User.GetName())
		return nil, trace.AccessDenied("user %q does not have permission to evaluate SSH access", authzContext.User.GetName())
	}

	return s.pdp.EvaluateSSHAccess(ctx, req)
}

func (s *Service) EvaluateSSHJoin(ctx context.Context, req *decisionpb.EvaluateSSHJoinRequest) (*decisionpb.EvaluateSSHJoinResponse, error) {
	authzContext, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.HasBuiltinRole(*authzContext, string(types.RoleAdmin)) {
		s.logger.WarnContext(ctx, "user does not have permission to evaluate SSH session-joining", "user", authzContext.User.GetName())
		return nil, trace.AccessDenied("user %q does not have permission to evaluate SSH session-joining", authzContext.User.GetName())
	}

	return s.UnimplementedDecisionServiceServer.EvaluateSSHJoin(ctx, req)
}

func (s *Service) EvaluateDatabaseAccess(ctx context.Context, req *decisionpb.EvaluateDatabaseAccessRequest) (*decisionpb.EvaluateDatabaseAccessResponse, error) {
	authzContext, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.HasBuiltinRole(*authzContext, string(types.RoleAdmin)) {
		s.logger.WarnContext(ctx, "user does not have permission to evaluate database access", "user", authzContext.User.GetName())
		return nil, trace.AccessDenied("user %q does not have permission to evaluate database access", authzContext.User.GetName())
	}

	return s.UnimplementedDecisionServiceServer.EvaluateDatabaseAccess(ctx, req)
}
