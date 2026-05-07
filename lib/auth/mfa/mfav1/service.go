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
	"context"
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
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

// Service implements the teleport.decision.v1alpha1.DecisionService gRPC API.
type Service struct {
	mfav1.UnimplementedMFAServiceServer

	logger     *slog.Logger
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
		logger:     slog.With(teleport.ComponentKey, "mfa.service"),
		authorizer: cfg.Authorizer,
		authServer: cfg.AuthServer,
	}, nil
}

// CompleteBrowserMFAChallenge takes a MFA response from the browser and returns
// it via an encrypted response parameter in a callback URL for the browser to
// return to tsh.
func (s *Service) CompleteBrowserMFAChallenge(ctx context.Context, req *mfav1.CompleteBrowserMFAChallengeRequest) (*mfav1.CompleteBrowserMFAChallengeResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.IsLocalOrRemoteUser(*authCtx) {
		return nil, trace.AccessDenied("only local or remote users can complete a browser MFA challenge")
	}

	if req.BrowserMfaResponse == nil {
		return nil, trace.BadParameter("missing browser_mfa_response in request")
	}

	if req.BrowserMfaResponse.RequestId == "" {
		return nil, trace.BadParameter("missing request_id in browser_mfa_response")
	}

	if req.BrowserMfaResponse.WebauthnResponse == nil {
		return nil, trace.BadParameter("missing webauthn_response in browser_mfa_response")
	}

	tshRedirectURL, err := s.authServer.CompleteBrowserMFAChallenge(
		ctx,
		req.BrowserMfaResponse.RequestId,
		req.BrowserMfaResponse.WebauthnResponse,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &mfav1.CompleteBrowserMFAChallengeResponse{
		TshRedirectUrl: tshRedirectURL,
	}, nil
}
