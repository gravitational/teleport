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

package joining

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	joiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	scopedjoiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/defaults"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

// Config contains the parameters for [New].
type Config struct {
	ScopedAuthorizer authz.ScopedAuthorizer
	Logger           *slog.Logger
	Backend          services.ScopedTokenService
}

// Server is the [scopedjoiningv1.ScopedJoiningServiceServer] returned by [New].
type Server struct {
	scopedjoiningv1.UnsafeScopedJoiningServiceServer

	authorizer authz.ScopedAuthorizer
	logger     *slog.Logger
	backend    services.ScopedTokenService
}

// New returns the auth server implementation for the scoped provisioning
// service, including the gRPC interface, authz enforcement, and business logic.
func New(c Config) (*Server, error) {
	if c.ScopedAuthorizer == nil {
		return nil, trace.BadParameter("missing Authorizer")
	}

	if c.Backend == nil {
		return nil, trace.BadParameter("missing Backend")
	}

	if c.Logger == nil {
		c.Logger = slog.With(teleport.ComponentKey, "scopes")
	}

	return &Server{
		authorizer: c.ScopedAuthorizer,
		logger:     c.Logger,
		backend:    c.Backend,
	}, nil
}

// CreateScopedToken implements [scopedjoiningv1.ScopedJoiningServiceServer].
func (s *Server) CreateScopedToken(ctx context.Context, req *scopedjoiningv1.CreateScopedTokenRequest) (*scopedjoiningv1.CreateScopedTokenResponse, error) {
	authzContext, err := s.authorizer.AuthorizeScoped(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ruleCtx := authzContext.RuleContext()
	if err := authzContext.CheckerContext.Decision(ctx, req.GetToken().GetScope(), func(checker *services.SplitAccessChecker) error {
		return checker.Common().CheckAccessToRules(&ruleCtx, scopedaccess.KindScopedToken, types.VerbCreate)
	}); err != nil {
		s.logger.WarnContext(ctx, "user does not have permission to create scoped tokens in the requested scope", "user", authzContext.User.GetName(), "scope", req.GetToken().GetScope())
		return nil, trace.Wrap(err)
	}

	token := req.GetToken()
	if token.GetMetadata().GetName() == "" {
		if token.Metadata == nil {
			token.Metadata = &headerv1.Metadata{}
		}
		name, err := utils.CryptoRandomHex(defaults.TokenLenBytes)
		if err != nil {
			return nil, trace.Wrap(err, "generating token value")
		}
		token.Metadata.Name = name
	}

	if token.GetSpec() != nil && token.GetSpec().GetJoinMethod() == "" {
		token.Spec.JoinMethod = string(types.JoinMethodToken)
	}

	res, err := s.backend.CreateScopedToken(ctx, req)
	return res, trace.Wrap(err)
}

// DeleteScopedToken implements [scopedjoiningv1.ScopedJoiningServiceServer].
func (s *Server) DeleteScopedToken(ctx context.Context, req *scopedjoiningv1.DeleteScopedTokenRequest) (*scopedjoiningv1.DeleteScopedTokenResponse, error) {
	authzContext, err := s.authorizer.AuthorizeScoped(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ruleCtx := authzContext.RuleContext()
	if err := authzContext.CheckerContext.CheckMaybeHasAccessToRules(&ruleCtx, scopedaccess.KindScopedToken, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	// if we're using unscoped credentials, just delete the scope token instead of trying to fetch its scope
	if authzContext.CheckerContext.Scoped() == nil {
		res, err := s.backend.DeleteScopedToken(ctx, req)
		return res, trace.Wrap(err)
	}

	getRes, err := s.backend.GetScopedToken(ctx, &scopedjoiningv1.GetScopedTokenRequest{
		Name: req.GetName(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authzContext.CheckerContext.Decision(ctx, getRes.GetToken().GetScope(), func(checker *services.SplitAccessChecker) error {
		return checker.Common().CheckAccessToRules(&ruleCtx, scopedaccess.KindScopedToken, types.VerbDelete)
	}); err != nil {
		s.logger.WarnContext(ctx, "user does not have permission to delete scoped tokens in the requested scope", "user", authzContext.User.GetName(), "scope", getRes.GetToken().GetScope())
		return nil, trace.Wrap(err)
	}

	res, err := s.backend.DeleteScopedToken(ctx, req)
	return res, trace.Wrap(err)
}

// GetScopedToken implements [scopedjoiningv1.ScopedJoiningServiceServer].
func (s *Server) GetScopedToken(ctx context.Context, req *scopedjoiningv1.GetScopedTokenRequest) (*scopedjoiningv1.GetScopedTokenResponse, error) {
	authzContext, err := s.authorizer.AuthorizeScoped(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ruleCtx := authzContext.RuleContext()
	if err := authzContext.CheckerContext.CheckMaybeHasAccessToRules(&ruleCtx, scopedaccess.KindScopedToken, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	res, err := s.backend.GetScopedToken(ctx, &scopedjoiningv1.GetScopedTokenRequest{
		Name: req.GetName(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authzContext.CheckerContext.Decision(ctx, res.GetToken().GetScope(), func(checker *services.SplitAccessChecker) error {
		return checker.Common().CheckAccessToRules(&ruleCtx, scopedaccess.KindScopedToken, types.VerbDelete)
	}); err != nil {
		s.logger.WarnContext(ctx, "user does not have permission to read scoped tokens in the requested scope", "user", authzContext.User.GetName(), "scope", res.GetToken().GetScope())
		return nil, trace.Wrap(err)
	}

	return res, trace.Wrap(err)
}

// ListScopedTokens implements [scopedjoiningv1.ScopedJoiningServiceServer].
func (s *Server) ListScopedTokens(ctx context.Context, req *scopedjoiningv1.ListScopedTokensRequest) (*scopedjoiningv1.ListScopedTokensResponse, error) {
	authzContext, err := s.authorizer.AuthorizeScoped(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ruleCtx := authzContext.RuleContext()
	if err := authzContext.CheckerContext.CheckMaybeHasAccessToRules(&ruleCtx, scopedaccess.KindScopedToken, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	res, err := s.backend.ListScopedTokens(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var authorizedTokens []*joiningv1.ScopedToken
	for _, token := range res.GetTokens() {
		if err := authzContext.CheckerContext.Decision(ctx, token.GetScope(), func(checker *services.SplitAccessChecker) error {
			return checker.Common().CheckAccessToRules(&ruleCtx, scopedaccess.KindScopedToken, types.VerbList)
		}); err != nil {
			s.logger.DebugContext(ctx, "user not authorized to access scoped token", "user", authzContext.User.GetName(), "scope", token.GetScope())
			continue
		}
		authorizedTokens = append(authorizedTokens, token)
	}
	res.Tokens = authorizedTokens
	return res, trace.Wrap(err)
}

// UpdateScopedToken implements [scopedjoiningv1.ScopedJoiningServiceServer].
func (s *Server) UpdateScopedToken(ctx context.Context, req *scopedjoiningv1.UpdateScopedTokenRequest) (*scopedjoiningv1.UpdateScopedTokenResponse, error) {
	return nil, trace.NotImplemented("scoped tokens must be recreated, they cannot be updated")
}
