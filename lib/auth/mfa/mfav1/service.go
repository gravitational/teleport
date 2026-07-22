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

// Package mfav1 contains the deprecated MFA v1 service. Only CompleteBrowserMFAChallenge remains live.
package mfav1

import (
	"context"

	"github.com/gravitational/trace"

	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	webauthnpb "github.com/gravitational/teleport/api/types/webauthn"
	"github.com/gravitational/teleport/lib/authz"
)

// AuthServer defines the subset of lib/auth.Server methods used by the MFA service.
type AuthServer interface {
	CompleteBrowserMFAChallenge(
		ctx context.Context,
		requestID string,
		webauthnResponse *webauthnpb.CredentialAssertionResponse,
	) (string, error)
}

// ServiceConfig holds creation parameters for [Service].
type ServiceConfig struct {
	Authorizer authz.Authorizer
	AuthServer AuthServer
}

// Service implements the mfav1.MFAServiceServer gRPC API.
type Service struct {
	mfav1.UnimplementedMFAServiceServer

	authorizer authz.Authorizer
	authServer AuthServer
}

// NewService creates a new [Service] instance.
func NewService(cfg ServiceConfig) (*Service, error) {
	switch {
	case cfg.Authorizer == nil:
		return nil, trace.BadParameter("param Authorizer is required for MFA service")
	case cfg.AuthServer == nil:
		return nil, trace.BadParameter("param AuthServer is required for MFA service")
	}

	return &Service{
		authorizer: cfg.Authorizer,
		authServer: cfg.AuthServer,
	}, nil
}

// CompleteBrowserMFAChallenge completes a browser MFA challenge.
//
//nolint:staticcheck // TODO(danielashare): Delete when Browser MFA has migrated to mfav2.
func (s *Service) CompleteBrowserMFAChallenge(ctx context.Context, req *mfav1.CompleteBrowserMFAChallengeRequest) (*mfav1.CompleteBrowserMFAChallengeResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.IsLocalOrRemoteUser(*authCtx) {
		return nil, trace.AccessDenied("only local or remote users can complete a browser MFA challenge")
	}

	if req.GetBrowserMfaResponse() == nil {
		return nil, trace.BadParameter("missing browser_mfa_response in request")
	}

	if req.GetBrowserMfaResponse().GetRequestId() == "" {
		return nil, trace.BadParameter("missing request_id in browser_mfa_response")
	}

	if req.GetBrowserMfaResponse().GetWebauthnResponse() == nil {
		return nil, trace.BadParameter("missing webauthn_response in browser_mfa_response")
	}

	tshRedirectURL, err := s.authServer.CompleteBrowserMFAChallenge(
		ctx,
		req.GetBrowserMfaResponse().GetRequestId(),
		req.GetBrowserMfaResponse().GetWebauthnResponse(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &mfav1.CompleteBrowserMFAChallengeResponse{
		TshRedirectUrl: tshRedirectURL,
	}, nil
}
