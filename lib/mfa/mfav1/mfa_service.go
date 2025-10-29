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

package mfav1

import (
	"cmp"
	"context"
	"log/slog"

	"github.com/gravitational/trace"

	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/mfa"
)

// ServiceConfig holds creation parameters for [Service].
type ServiceConfig struct {
	MFAService *mfa.Service

	// Authorizer used by the service.
	Authorizer authz.Authorizer
	Logger     *slog.Logger
}

// Service implements the teleport.decision.v1alpha1.DecisionService gRPC API.
type Service struct {
	mfav1.UnimplementedMFAServiceServer

	mfa        *mfa.Service
	authorizer authz.Authorizer
	logger     *slog.Logger
}

// NewService creates a new [Service] instance.
func NewService(cfg ServiceConfig) (*Service, error) {
	if cfg.MFAService == nil {
		return nil, trace.BadParameter("param MFAService required")
	}

	if cfg.Authorizer == nil {
		return nil, trace.BadParameter("param Authorizer required")
	}

	return &Service{
		mfa:        cfg.MFAService,
		authorizer: cfg.Authorizer,
		logger:     cmp.Or(cfg.Logger, slog.Default()),
	}, nil
}

// CreateChallengeForAction creates and returns an MFA challenge for a specific action.
func (s *Service) CreateChallengeForAction(ctx context.Context, req *mfav1.CreateChallengeRequest) (*mfav1.CreateChallengeResponse, error) {
	authzContext, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.IsLocalOrRemoteUser(*authzContext) {
		s.logger.WarnContext(ctx, "user is not allowed to create MFA challenges", "user", authzContext.User.GetName())
		return nil, trace.AccessDenied("only end users can create MFA challenges", authzContext.User.GetName())
	}

	return s.mfa.CreateChallengeForAction(ctx, req)
}

// ValidateChallengeForAction validates the MFA challenge response provided by the user for a specific user action.
func (s *Service) ValidateChallengeForAction(ctx context.Context, req *mfav1.ValidateChallengeRequest) (*mfav1.ValidateChallengeResponse, error) {
	authzContext, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.HasBuiltinRole(*authzContext, string(types.RoleNode)) {
		s.logger.WarnContext(ctx, "user does not have permission to validate MFA challenge", "user", authzContext.User.GetName())
		return nil, trace.AccessDenied("user %q does not have permission to validate MFA challenge", authzContext.User.GetName())
	}

	return s.mfa.ValidateChallengeForAction(ctx, req)
}
