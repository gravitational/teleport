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
	"errors"
	"slices"
	"time"

	"github.com/gravitational/trace"

	joiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/scopes/joining"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
	"github.com/gravitational/teleport/lib/utils/lockmap"
)

const (
	scopedTokenPrefix = "scoped_token"
)

// ScopedTokenService exposes backend functionality for working with scoped token resources.
type ScopedTokenService struct {
	svc        *generic.ServiceWrapper[*joiningv1.ScopedToken]
	tokenLocks lockmap.LockMap[string]
}

// NewScopedTokenService creates a new ScopedTokenService.
func NewScopedTokenService(b backend.Backend) (*ScopedTokenService, error) {
	const pageLimit = 100
	svc, err := generic.NewServiceWrapper(generic.ServiceConfig[*joiningv1.ScopedToken]{
		Backend:       b,
		PageLimit:     pageLimit,
		ResourceKind:  types.KindScopedToken,
		BackendPrefix: backend.NewKey(scopedTokenPrefix),
		MarshalFunc:   services.MarshalProtoResource[*joiningv1.ScopedToken],
		UnmarshalFunc: services.UnmarshalProtoResource[*joiningv1.ScopedToken],
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &ScopedTokenService{
		svc: svc,
	}, nil
}

// CreateScopedToken adds a scoped token to the auth server.
func (s *ScopedTokenService) CreateScopedToken(ctx context.Context, req *joiningv1.CreateScopedTokenRequest) (*joiningv1.CreateScopedTokenResponse, error) {
	if err := joining.StrongValidateToken(req.GetToken()); err != nil {
		return nil, trace.Wrap(err)
	}

	// status can not be explicitly assigned during creation
	req.Token.Status = &joiningv1.ScopedTokenStatus{}

	created, err := s.svc.CreateResource(ctx, req.GetToken())
	return &joiningv1.CreateScopedTokenResponse{
		Token: created,
	}, trace.Wrap(err)
}

// GetScopedToken finds and returns a scoped token by name.
func (s *ScopedTokenService) GetScopedToken(ctx context.Context, req *joiningv1.GetScopedTokenRequest) (*joiningv1.GetScopedTokenResponse, error) {
	token, err := s.svc.GetResource(ctx, req.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := joining.WeakValidateToken(token); err != nil {
		return nil, trace.Wrap(err)
	}
	return &joiningv1.GetScopedTokenResponse{Token: token}, nil
}

// UseScopedToken fetches a scoped join token by unique name and checks if it
// can be used for provisioning. Expired tokens will be deleted.
func (s *ScopedTokenService) UseScopedToken(ctx context.Context, name string) (*joiningv1.ScopedToken, error) {
	token, err := s.svc.GetResource(ctx, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := joining.ValidateTokenForUse(token); err != nil {
		// unscoped tokens are automatically deleted on use after expiration,
		// so we do the same here for parity
		if errors.Is(err, joining.ErrTokenExpired) {
			if err := s.svc.DeleteResource(ctx, name); err != nil {
				return nil, trace.Wrap(err, "cleaning up expired token")
			}
		}
		return nil, trace.Wrap(err)
	}
	return token, nil
}

// refetchToken will refetch the given scoped token from the backend and ensure
// that it has not materially changed.
func (s *ScopedTokenService) refetchToken(ctx context.Context, token *joiningv1.ScopedToken) (*joiningv1.ScopedToken, error) {
	tok, err := s.svc.GetResource(ctx, token.GetMetadata().GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if tok.GetSpec().GetAssignedScope() != token.GetSpec().GetAssignedScope() {
		return nil, trace.BadParameter("re-fetched scoped token does not assign to the expected scope")
	}

	if tok.GetSpec().GetJoinMethod() != token.GetSpec().GetJoinMethod() {
		return nil, trace.BadParameter("re-fetched scoped token does not use the expected join method")
	}

	roles, err := types.NewTeleportRoles(token.GetSpec().GetRoles())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	fetchedRoles, err := types.NewTeleportRoles(tok.GetSpec().GetRoles())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !roles.Equals(fetchedRoles) {
		return nil, trace.BadParameter("re-fetched scoped token does not assign the expected roles")
	}

	return tok, nil
}

// ConsumeScopedToken consumes a usage of a scoped token. A
// [*trace.LimitExceededError] is returned if the token is expired or has no
// remaining uses, which should be treated as a failure to provision.
func (s *ScopedTokenService) ConsumeScopedToken(ctx context.Context, token *joiningv1.ScopedToken, publicKey []byte) (*joiningv1.ScopedToken, error) {
	token, err := s.refetchToken(ctx, token)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := joining.ValidateTokenForUse(token); err != nil {
		return nil, trace.Wrap(err)
	}

	// we can short circuit here if this token has no limits
	if token.GetSpec().MaxUses == nil {
		return token, nil
	}

	name := token.GetMetadata().GetName()
	s.tokenLocks.Lock(name)
	defer s.tokenLocks.Unlock(name)

	if token.Status == nil {
		token.Status = &joiningv1.ScopedTokenStatus{}
	}

	// retrying from a previously successful attempt doesn't count against the
	// total, so we can short circuit without writing anything to the backend
	if len(publicKey) > 0 {
		for _, key := range token.GetStatus().GetPublicKeysProvisioned() {
			// no need to check usage count if this is a retry of a
			if slices.Equal(key, publicKey) {
				return token, nil
			}
		}
	}

	// the max number of attempts to consume a scoped token before failing a
	// join attempt
	const maxConsumeAttempts = 7

	// The max number of public keys that can be cached to support idempotent retries
	// using the same scoped token. As of writing, a scoped token can not set a limit
	// greater than 32. This limit is defined separately to prevent unintentionally
	// exploding cached public keys by increasing scoped token usage limits alone.
	const idempotencyLimit = 32

	// retry jitter to spread out attempts to consume scoped token
	jitter := func() time.Duration {
		return retryutils.HalfJitter(time.Second * 1)
	}

	// updates may fail due to revision changes from other auth instances, so we run
	// any required updates in a retry loop
	for range maxConsumeAttempts {
		if token.Status.AttemptedUses >= token.GetSpec().GetMaxUses() {
			return nil, trace.Wrap(joining.ErrTokenExhausted)
		}

		token.Status.AttemptedUses++
		if token.GetSpec().GetMaxUses() < idempotencyLimit {
			token.Status.PublicKeysProvisioned = append(token.Status.PublicKeysProvisioned, publicKey)
		}

		updated, err := s.svc.ConditionalUpdateResource(ctx, token)
		if err == nil {
			return updated, nil
		}
		if !errors.Is(err, backend.ErrIncorrectRevision) {
			return nil, trace.Wrap(err)
		}

		select {
		case <-ctx.Done():
			return nil, trace.Wrap(ctx.Err())
		case <-time.After(jitter()):
		}

		token, err = s.refetchToken(ctx, token)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return nil, trace.LimitExceeded("too many failed attempts to consume scoped token")
}

func evalScopeFilter(filter *scopesv1.Filter, scope string) bool {
	if filter == nil {
		return true
	}

	switch filter.Mode {
	case scopesv1.Mode_MODE_RESOURCES_SUBJECT_TO_SCOPE:
		return scopes.ResourceScope(scope).IsSubjectToPolicyScope(filter.Scope)
	case scopesv1.Mode_MODE_POLICIES_APPLICABLE_TO_SCOPE:
		return scopes.PolicyScope(scope).AppliesToResourceScope(filter.Scope)
	}

	return true
}

// ListScopedTokens retrieves a paginated list of scoped join tokens.
func (s *ScopedTokenService) ListScopedTokens(ctx context.Context, req *joiningv1.ListScopedTokensRequest) (*joiningv1.ListScopedTokensResponse, error) {
	// we only want to return filters if at least one of the filters
	// has been defined, otherwise we should return nil so that the
	// backend can choose to perform a simple list operation instead
	// of a list with filter
	switch {
	case req.ResourceScope != nil:
	case req.AssignedScope != nil:
	case len(req.Roles) > 0:
	case len(req.Labels) > 0:
	default:
		tokens, cursor, err := s.svc.ListResources(ctx, int(req.GetLimit()), req.GetCursor())

		if err != nil {
			return nil, trace.Wrap(err)
		}

		return &joiningv1.ListScopedTokensResponse{
			Tokens: tokens,
			Cursor: cursor,
		}, nil
	}

	if req.GetAssignedScope().GetScope() != "" {
		if err := scopes.WeakValidate(req.GetAssignedScope().GetScope()); err != nil {
			return nil, trace.BadParameter("invalid scope for assigned filter: %s", req.GetAssignedScope().GetScope())
		}

	}

	if req.GetResourceScope().GetScope() != "" {
		if err := scopes.WeakValidate(req.GetResourceScope().GetScope()); err != nil {
			return nil, trace.BadParameter("invalid scope for resource filter: %s", req.GetResourceScope().GetScope())
		}
	}

	filterRoles, err := types.NewTeleportRoles(req.GetRoles())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	filterFn := func(token *joiningv1.ScopedToken) bool {
		if len(req.GetRoles()) > 0 {
			roles, err := types.NewTeleportRoles(token.Spec.Roles)
			if err != nil {
				return false
			}

			if !filterRoles.IncludeAny(roles...) {
				return false
			}
		}

		if !evalScopeFilter(req.GetAssignedScope(), token.Spec.AssignedScope) {
			return false
		}

		if !evalScopeFilter(req.GetResourceScope(), token.Scope) {
			return false
		}

		for k, v := range req.GetLabels() {
			if token.GetMetadata().GetLabels()[k] != v {
				return false
			}
		}

		if err := joining.WeakValidateToken(token); err != nil {
			return false
		}

		return true
	}

	tokens, cursor, err := s.svc.ListResourcesWithFilter(ctx, int(req.GetLimit()), req.GetCursor(), filterFn)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &joiningv1.ListScopedTokensResponse{
		Tokens: tokens,
		Cursor: cursor,
	}, nil
}

// DeleteScopedToken deletes a scoped token by name.
func (s *ScopedTokenService) DeleteScopedToken(ctx context.Context, req *joiningv1.DeleteScopedTokenRequest) (*joiningv1.DeleteScopedTokenResponse, error) {
	return nil, trace.Wrap(s.svc.DeleteResource(ctx, req.GetName()))
}

type scopedTokenParser struct {
	baseParser
}

func newScopedTokenParser() *scopedTokenParser {
	return &scopedTokenParser{
		baseParser: newBaseParser(backend.NewKey(scopedTokenPrefix)),
	}
}

func (p *scopedTokenParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpPut:
		scopedToken, err := services.UnmarshalProtoResource[*joiningv1.ScopedToken](
			event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err, "unmarshaling resource from event")
		}
		return types.Resource153ToLegacy(scopedToken), nil
	case types.OpDelete:
		name := event.Item.Key.TrimPrefix(backend.NewKey(scopedTokenPrefix)).String()
		if name == "" {
			return nil, trace.NotFound("failed parsing %v", event.Item.Key)
		}
		return &types.ResourceHeader{
			Kind:    types.KindScopedToken,
			Version: types.V1,
			Metadata: types.Metadata{
				Name: name,
			},
		}, nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}
