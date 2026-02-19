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
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"iter"
	"log/slog"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"

	"github.com/gravitational/teleport"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	scopedjoiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/scopes"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

const defaultTokenPageSize = 100

// Config contains the parameters for [New].
type Config struct {
	ScopedAuthorizer authz.ScopedAuthorizer
	Logger           *slog.Logger
	Backend          services.ScopedTokenService
	MaxPageSize      int
}

// Server is the [scopedjoiningv1.ScopedJoiningServiceServer] returned by [New].
type Server struct {
	scopedjoiningv1.UnsafeScopedJoiningServiceServer

	authorizer  authz.ScopedAuthorizer
	logger      *slog.Logger
	backend     services.ScopedTokenService
	maxPageSize uint32
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
		authorizer:  c.ScopedAuthorizer,
		logger:      c.Logger,
		backend:     c.Backend,
		maxPageSize: cmp.Or(uint32(c.MaxPageSize), defaultTokenPageSize),
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
		name, err := uuid.NewRandom()
		if err != nil {
			return nil, trace.Wrap(err, "generating token name")
		}
		token.Metadata.Name = name.String()
	}

	if token.GetSpec() != nil && token.GetSpec().GetJoinMethod() == "" {
		token.Spec.JoinMethod = string(types.JoinMethodToken)
	}

	if token.GetSpec().GetJoinMethod() == string(types.JoinMethodToken) {
		if token.Status == nil {
			token.Status = &scopedjoiningv1.ScopedTokenStatus{}
		}
		secret, err := utils.CryptoRandomHex(defaults.TokenLenBytes)
		if err != nil {
			return nil, trace.Wrap(err, "generating token secret")
		}
		token.Status.Secret = secret
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

	// perform an early check for root scope delete permission to allow us to short-circuit
	// and perform an unconditional delete. this is not strictly necessary, but allows us to
	// have an escape hatch for deleting tokens that are so malformed that they cannot be read.
	if err := authzContext.CheckerContext.Decision(ctx, scopes.Root, func(checker *services.SplitAccessChecker) error {
		return checker.Common().CheckAccessToRules(&ruleCtx, scopedaccess.KindScopedRole, types.VerbDelete)
	}); err == nil {
		return s.backend.DeleteScopedToken(ctx, req)
	}

	// fetch the token so we can determine the resource scope
	preAuthzRes, err := s.backend.GetScopedToken(ctx, &scopedjoiningv1.GetScopedTokenRequest{
		Name: req.GetName(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authzContext.CheckerContext.Decision(ctx, preAuthzRes.GetToken().GetScope(), func(checker *services.SplitAccessChecker) error {
		return checker.Common().CheckAccessToRules(&ruleCtx, scopedaccess.KindScopedToken, types.VerbDelete)
	}); err != nil {
		s.logger.WarnContext(ctx, "user does not have permission to delete scoped tokens in the requested scope", "user", authzContext.User.GetName(), "scope", preAuthzRes.GetToken().GetScope())
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

	readVerb := types.VerbReadNoSecrets
	if req.GetWithSecret() {
		readVerb = types.VerbRead
	}

	ruleCtx := authzContext.RuleContext()
	if err := authzContext.CheckerContext.CheckMaybeHasAccessToRules(&ruleCtx, scopedaccess.KindScopedToken, readVerb); err != nil {
		return nil, trace.Wrap(err)
	}

	preAuthzRes, err := s.backend.GetScopedToken(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authzContext.CheckerContext.Decision(ctx, preAuthzRes.GetToken().GetScope(), func(checker *services.SplitAccessChecker) error {
		return checker.Common().CheckAccessToRules(&ruleCtx, scopedaccess.KindScopedToken, readVerb)
	}); err != nil {
		s.logger.WarnContext(ctx, "user does not have permission to read scoped tokens in the requested scope", "user", authzContext.User.GetName(), "scope", preAuthzRes.GetToken().GetScope())
		return nil, trace.Wrap(err)
	}

	res := preAuthzRes
	return res, trace.Wrap(err)
}

func makeCursor(token *scopedjoiningv1.ScopedToken) string {
	if token == nil {
		return ""
	}
	hash := sha256.Sum256([]byte(token.GetMetadata().GetName()))
	return base64.StdEncoding.EncodeToString(hash[:])
}

func (s *Server) scopedTokenIter(ctx context.Context, req *scopedjoiningv1.ListScopedTokensRequest) iter.Seq2[*scopedjoiningv1.ScopedToken, error] {
	return func(yield func(token *scopedjoiningv1.ScopedToken, err error) bool) {
		iterReq := proto.CloneOf(req)
		iterReq.Limit = s.maxPageSize

		var cursorFound bool
		for {
			res, err := s.backend.ListScopedTokens(ctx, iterReq)
			if err != nil {
				if !yield(nil, trace.Wrap(err)) {
					return
				}
			}

			for _, tok := range res.GetTokens() {
				if !cursorFound && req.GetCursor() != "" && makeCursor(tok) != req.GetCursor() {
					continue
				}
				cursorFound = true
				if !yield(tok, nil) {
					return
				}
			}

			// make sure we stop when we reach the end
			if res.GetCursor() == "" {
				return
			}
			iterReq.Cursor = res.GetCursor()
		}
	}
}

// ListScopedTokens implements [scopedjoiningv1.ScopedJoiningServiceServer].
func (s *Server) ListScopedTokens(ctx context.Context, req *scopedjoiningv1.ListScopedTokensRequest) (*scopedjoiningv1.ListScopedTokensResponse, error) {
	authzContext, err := s.authorizer.AuthorizeScoped(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	readVerb := types.VerbReadNoSecrets
	if req.GetWithSecrets() {
		readVerb = types.VerbRead
	}

	ruleCtx := authzContext.RuleContext()
	if err := authzContext.CheckerContext.CheckMaybeHasAccessToRules(&ruleCtx, scopedaccess.KindScopedToken, readVerb, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	limit := int(req.GetLimit())
	if limit == 0 {
		limit = defaultTokenPageSize
	}

	var authorizedTokens []*scopedjoiningv1.ScopedToken
	for token, err := range s.scopedTokenIter(ctx, req) {
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if err := authzContext.CheckerContext.Decision(ctx, token.GetScope(), func(checker *services.SplitAccessChecker) error {
			return checker.Common().CheckAccessToRules(&ruleCtx, scopedaccess.KindScopedToken, readVerb, types.VerbList)
		}); err != nil {
			continue
		}
		authorizedTokens = append(authorizedTokens, token)

		// stop once we've fulfilled the requested page size
		if len(authorizedTokens) >= limit {
			break
		}
	}

	var lastToken *scopedjoiningv1.ScopedToken
	if len(authorizedTokens) >= limit {
		lastToken = authorizedTokens[len(authorizedTokens)-1]
	}
	return &scopedjoiningv1.ListScopedTokensResponse{
		Tokens: authorizedTokens,
		Cursor: makeCursor(lastToken),
	}, nil
}

// UpsertScopedToken implements [scopedjoiningv1.ScopedJoiningServiceServer].
func (s *Server) UpsertScopedToken(ctx context.Context, req *scopedjoiningv1.UpsertScopedTokenRequest) (*scopedjoiningv1.UpsertScopedTokenResponse, error) {
	authzContext, err := s.authorizer.AuthorizeScoped(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// We rely on the backend guarantee that scoped tokens updates won't overwrite an existing token if it has a different scope, usage mode, or secret.
	ruleCtx := authzContext.RuleContext()
	if err := authzContext.CheckerContext.Decision(ctx, req.GetToken().GetScope(), func(checker *services.SplitAccessChecker) error {
		return checker.Common().CheckAccessToRules(&ruleCtx, scopedaccess.KindScopedToken, types.VerbUpdate, types.VerbCreate)
	}); err != nil {
		s.logger.WarnContext(ctx, "user does not have permission to upsert scoped tokens in the requested scope", "user", authzContext.User.GetName(), "scope", req.GetToken().GetScope())
		return nil, trace.Wrap(err)
	}

	res, err := s.backend.UpsertScopedToken(ctx, req)
	return res, trace.Wrap(err)
}
