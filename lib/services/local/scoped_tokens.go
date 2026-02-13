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
	"cmp"
	"context"
	"crypto/sha256"
	"errors"
	"slices"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	joiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/scopes/joining"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

const (
	scopedTokenPrefix      = "scoped_token"
	maxTokenUpsertAttempts = 4
)

// ScopedTokenService exposes backend functionality for working with scoped token resources.
type ScopedTokenService struct {
	svc     *generic.ServiceWrapper[*joiningv1.ScopedToken]
	backend backend.Backend
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
		svc:     svc,
		backend: b,
	}, nil
}

func itemFromScopedToken(token *joiningv1.ScopedToken) (backend.Item, error) {
	key := backend.NewKey(scopedTokenPrefix, token.GetMetadata().GetName())
	value, err := services.MarshalProtoResource(token)
	if err != nil {
		return backend.Item{}, trace.Wrap(err)
	}

	// need to make sure expires is the zero value of [time.Time] unless an
	// expiry is explicitly set, otherwise the backend item will be created
	// in an expired state
	var expires time.Time
	if ex := token.GetMetadata().GetExpires(); ex != nil {
		expires = ex.AsTime()
	}
	return backend.Item{
		Key:      key,
		Value:    value,
		Expires:  expires,
		Revision: token.GetMetadata().GetRevision(),
	}, nil
}

// CreateScopedToken adds a scoped token to the auth server.
func (s *ScopedTokenService) CreateScopedToken(ctx context.Context, req *joiningv1.CreateScopedTokenRequest) (*joiningv1.CreateScopedTokenResponse, error) {
	if err := joining.StrongValidateToken(req.GetToken()); err != nil {
		return nil, trace.Wrap(err)
	}

	item, err := itemFromScopedToken(req.GetToken())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	revision, err := s.backend.AtomicWrite(ctx, []backend.ConditionalAction{
		{
			Key:       backend.NewKey(scopedTokenPrefix, req.GetToken().GetMetadata().GetName()),
			Condition: backend.NotExists(),
			Action:    backend.Put(item),
		},
		{
			Key:       backend.NewKey(tokensPrefix, req.GetToken().GetMetadata().GetName()),
			Condition: backend.NotExists(),
			// the second action is a no-op because we only need to
			// execute a single action to create the scoped token,
			// but both conditions must be met
			Action: backend.Nop(),
		},
	})
	if err != nil {
		if errors.Is(err, backend.ErrConditionFailed) {
			return nil, trace.AlreadyExists("scoped token could not be created due to name conflict with an existing scoped or unscoped token, please try again with a different name or delete the conflicting token")
		}
		return nil, trace.Wrap(err)
	}

	created := proto.CloneOf(req.GetToken())
	created.Metadata.Revision = revision
	return &joiningv1.CreateScopedTokenResponse{
		Token: created,
	}, nil
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

	if !req.GetWithSecret() {
		setScopedTokenWithoutSecret(token)
	}
	return &joiningv1.GetScopedTokenResponse{Token: token}, nil
}

// tokenReuseDuration is how long a scoped token can be reused by the host that consumed it
const tokenReuseDuration = time.Minute * 30

// UseScopedToken attempts to use a scoped token to provision a resource. A
// [trace.LimitExceededError] error is returned if the token is expired or has
// been exhausted, which should be treated as a failure to provision. The
// given public key is an idempotency key to allow the same host to temporarily
// retry a failed join due to spurious errors even after the token has been
// consumed.
func (s *ScopedTokenService) UseScopedToken(ctx context.Context, token *joiningv1.ScopedToken, publicKey []byte) (*joiningv1.ScopedToken, error) {
	if err := joining.ValidateTokenForUse(token); err != nil {
		return nil, trace.Wrap(err)
	}

	if joining.TokenUsageMode(token.Spec.UsageMode) != joining.TokenUsageModeSingle {
		return token, nil
	}

	// make sure the correct, non-nil usage status is set
	token.Status = cmp.Or(token.Status, &joiningv1.ScopedTokenStatus{})
	token.Status.Usage = cmp.Or(token.Status.Usage, &joiningv1.UsageStatus{})
	if token.Status.Usage.Status == nil {
		token.Status.Usage.Status = &joiningv1.UsageStatus_SingleUse{
			SingleUse: &joiningv1.SingleUseStatus{},
		}
	}
	usage := token.Status.Usage.GetSingleUse()
	if usage == nil {
		return nil, trace.Errorf("single use token does not have a single use status")
	}

	fp := sha256.Sum256(publicKey)
	if len(usage.GetUsedByFingerprint()) > 0 {
		// no need to update the token if we're retrying for the same public key
		if slices.Equal(usage.GetUsedByFingerprint(), fp[:]) {
			return token, nil
		}

		return nil, trace.Wrap(joining.ErrTokenExhausted)
	}

	// set the public key if this is the first time this token has been used
	usage.UsedByFingerprint = fp[:]
	usage.UsedAt = timestamppb.New(time.Now())
	usage.ReusableUntil = timestamppb.New(time.Now().Add(tokenReuseDuration))

	token, err := s.svc.ConditionalUpdateResource(ctx, token)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return token, nil
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
		if !req.GetWithSecrets() {
			setScopedTokenWithoutSecret(tokens...)
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

	if !req.GetWithSecrets() {
		setScopedTokenWithoutSecret(tokens...)
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

// UpsertScopedToken updates or creates a scoped token. If updating an existing token, the scope and status must not be modified.
func (s *ScopedTokenService) UpsertScopedToken(ctx context.Context, req *joiningv1.UpsertScopedTokenRequest) (*joiningv1.UpsertScopedTokenResponse, error) {
	tokenUpsert := req.GetToken()

	if err := joining.StrongValidateToken(tokenUpsert); err != nil {
		return nil, trace.Wrap(err)
	}

	// We handle 4 retry attempts to try and handle some concurrency. Handling more retries than this
	// indicates that something may be going wrong.
	for attempt := range maxTokenUpsertAttempts {
		if attempt != 0 {
			select {
			case <-time.After(retryutils.FullJitter(time.Duration(300*attempt) * time.Millisecond)):
			case <-ctx.Done():
				return nil, trace.Wrap(ctx.Err())
			}
		}

		existingToken, err := s.svc.GetResource(ctx, tokenUpsert.GetMetadata().GetName())
		if err != nil {
			if !trace.IsNotFound(err) {
				return nil, trace.Wrap(err)
			}
		}

		// We enforce this validating the updates here in order for the access-control layer's checks to be sound.
		// Changing this would require rethinking or additional changes to the access-control checks.
		if err := joining.ValidateTokenUpdate(existingToken, tokenUpsert); err != nil {
			return nil, trace.Wrap(err)
		}

		if existingToken != nil {
			// Use conditional update with revision checking to ensure the token hasn't changed since we validated it.
			// This prevents race conditions where the token could be deleted and recreated with
			// different properties between our validation check and the write.
			tokenUpsert.GetMetadata().Revision = existingToken.GetMetadata().GetRevision()
			upsertedToken, err := s.svc.ConditionalUpdateResource(ctx, tokenUpsert)
			if err != nil {
				if trace.IsCompareFailed(err) {
					continue
				}
				return nil, trace.Wrap(err)
			}

			return &joiningv1.UpsertScopedTokenResponse{
				Token: upsertedToken,
			}, nil
		}

		createdToken, err := s.svc.CreateResource(ctx, tokenUpsert)
		if err != nil {
			// This will only be true if we call upsert concurrently and there was no existing token.
			// One of the concurrent calls will retry but as an update call.
			if trace.IsAlreadyExists(err) {
				continue
			}
			return nil, trace.Wrap(err)
		}

		return &joiningv1.UpsertScopedTokenResponse{
			Token: createdToken,
		}, nil
	}

	return nil, trace.LimitExceeded("exceeded max retries attempting to upsert scoped token - too many concurrent modifications")
}

func setScopedTokenWithoutSecret(token ...*joiningv1.ScopedToken) {
	for _, t := range token {
		if t != nil && t.Status != nil {
			t.Status.Secret = ""
		}
	}
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

type staticScopedTokenParser struct {
	baseParser
}

func newStaticScopedTokenParser() *staticScopedTokenParser {
	return &staticScopedTokenParser{
		baseParser: newBaseParser(backend.NewKey(clusterConfigPrefix, types.MetaNameStaticScopedTokens)),
	}
}

func (p *staticScopedTokenParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		return &types.ResourceHeader{
			Kind:    types.KindStaticScopedTokens,
			Version: types.V1,
			Metadata: types.Metadata{
				Name: types.MetaNameStaticScopedTokens,
			},
		}, nil
	case types.OpPut:
		tokens, err := services.UnmarshalProtoResource[*joiningv1.StaticScopedTokens](
			event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err, "unmarshaling resource from event")
		}
		return types.Resource153ToLegacy(tokens), nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}
