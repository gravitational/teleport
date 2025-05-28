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

package local

import (
	"context"

	"github.com/gravitational/trace"

	scopedtokenv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopedtoken/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/scopes/tokens"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

// ScopedTokenService manages backend state for the ScopedTokens.
type ScopedTokenService struct {
	scopedTokens *generic.ServiceWrapper[*scopedtokenv1.ScopedToken]
}

const (
	scopedTokenPrefix = "scoped_token"
)

// NewScopeTokenService creates a new ScopedTokenService for the specified backend.
func NewScopeTokenService(bk backend.Backend) (*ScopedTokenService, error) {
	s, err := generic.NewServiceWrapper(generic.ServiceConfig[*scopedtokenv1.ScopedToken]{
		Backend:       bk,
		ResourceKind:  tokens.KindScopedToken,
		BackendPrefix: backend.NewKey(scopedTokenPrefix),
		MarshalFunc:   services.MarshalProtoResource[*scopedtokenv1.ScopedToken],
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &ScopedTokenService{scopedTokens: s}, nil
}

func (s *ScopedTokenService) CreateScopedToken(ctx context.Context, token *scopedtokenv1.ScopedToken) (*scopedtokenv1.ScopedToken, error) {
	if token == nil {
		return nil, trace.BadParameter("missing scoped token in create request")
	}

	if err := tokens.StrongValidateToken(token); err != nil {
		return nil, trace.Wrap(err)
	}

	out, err := s.scopedTokens.CreateResource(ctx, token)
	return out, trace.Wrap(err)
}

func (s *ScopedTokenService) UpdateScopedToken(ctx context.Context, token *scopedtokenv1.ScopedToken) (*scopedtokenv1.ScopedToken, error) {
	if token == nil {
		return nil, trace.BadParameter("missing scoped token in update request")
	}

	if err := tokens.StrongValidateToken(token); err != nil {
		return nil, trace.Wrap(err)
	}

	out, err := s.scopedTokens.ConditionalUpdateResource(ctx, token)
	return out, trace.Wrap(err)
}

func (s *ScopedTokenService) DeleteScopedToken(ctx context.Context, name string) error {
	return trace.Wrap(s.scopedTokens.DeleteResource(ctx, name))
}

func (s *ScopedTokenService) GetScopedToken(ctx context.Context, name string) (*scopedtokenv1.ScopedToken, error) {
	if name == "" {
		return nil, trace.BadParameter("missing scoped token name in get request")
	}

	token, err := s.scopedTokens.GetResource(ctx, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := scopes.WeakValidateResource(token, tokens.KindScopedToken, types.V1); err != nil {
		return nil, trace.Wrap(err)
	}

	return token, nil
}

// StreamScopedRoles returns a stream of all scoped tokens in the backend. Malformed tokens are skipped. Returned tokens
// have had weak validation applied.
func (s *ScopedTokenService) ScopedTokens(ctx context.Context, pageSize int, pageToken string) stream.Stream[*scopedtokenv1.ScopedToken] {
	return func(yield func(*scopedtokenv1.ScopedToken, error) bool) {
		for token, err := range s.scopedTokens.Resources(ctx, pageToken, pageSize) {
			if err != nil {
				yield(nil, err)
				return
			}

			if err := scopes.WeakValidateResource(token, tokens.KindScopedToken, types.V1); err != nil {
				yield(nil, trace.Wrap(err))
				return
			}

			if !yield(token, nil) {
				return
			}
		}
	}
}
